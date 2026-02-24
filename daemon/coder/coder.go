package coder

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"

	"scaffold/sessionbus"
)

const (
	sessionID  = "scaffold-coder"
	runBaseDir = "/tmp/scaffold-coder"
	maxRecent  = 20
)

// Config holds coder configuration.
type Config struct {
	AllowedPaths  []string
	SkillPath     string // path to coder-skill.md
	StepSkillsDir string // path to coder-skills/ dir
}

// Coder listens for code_task messages on the session bus and runs chains.
type Coder struct {
	bus    *sessionbus.Bus
	hub    *SSEHub
	sem    chan struct{} // semaphore(1) — one chain at a time
	cfg    Config

	mu     sync.RWMutex
	active *Task
	recent []*Task
}

// Task tracks a running or completed chain.
type Task struct {
	ID        string     `json:"id"`
	Chain     string     `json:"chain"`
	TaskDesc  string     `json:"task"`
	CWD       string     `json:"cwd"`
	Status    string     `json:"status"` // running, done, failed, cancelled
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Steps     []StepInfo `json:"steps"`
	Summary   string     `json:"summary,omitempty"`
	FailedStep string    `json:"failed_step,omitempty"`
	Error     string     `json:"error,omitempty"`
	RunDir    string     `json:"run_dir"`
	ReplyTo   string     `json:"-"`

	cancel context.CancelFunc
}

// StepInfo is per-step status within a task.
type StepInfo struct {
	Name     string  `json:"name"`
	Status   string  `json:"status"` // pending, running, done, failed
	ElapsedS float64 `json:"elapsed_s,omitempty"`
}

// CodeTaskMessage is the session bus payload from agent → coder.
type CodeTaskMessage struct {
	Type    string `json:"type"`
	Task    string `json:"task"`
	Chain   string `json:"chain"`
	CWD     string `json:"cwd"`
	ReplyTo string `json:"reply_to"`
}

// CoderResultMessage is the session bus payload from coder → agent on completion.
type CoderResultMessage struct {
	Type       string  `json:"type"`
	TaskID     string  `json:"task_id"`
	Chain      string  `json:"chain"`
	Status     string  `json:"status"` // done, failed, cancelled
	Summary    string  `json:"summary,omitempty"`
	FailedStep string  `json:"failed_step,omitempty"`
	Error      string  `json:"error,omitempty"`
	ElapsedS   float64 `json:"elapsed_s"`
}

// New creates a Coder. Call Start(ctx) to begin consuming the bus.
func New(bus *sessionbus.Bus, cfg Config) *Coder {
	return &Coder{
		bus: bus,
		hub: newSSEHub(),
		sem: make(chan struct{}, 1),
		cfg: cfg,
	}
}

// Hub returns the SSE hub for API handler use.
func (c *Coder) Hub() *SSEHub {
	return c.hub
}

// Start begins the bus consumer loop in a goroutine.
func (c *Coder) Start(ctx context.Context) {
	go c.loop(ctx)
}

// ListTasks returns active + recent tasks (active first). Always returns a non-nil slice.
func (c *Coder) ListTasks() []*Task {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]*Task, 0)
	if c.active != nil {
		out = append(out, c.active)
	}
	out = append(out, c.recent...)
	return out
}

// GetTask looks up a task by ID.
func (c *Coder) GetTask(id string) (*Task, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.active != nil && c.active.ID == id {
		return c.active, true
	}
	for _, t := range c.recent {
		if t.ID == id {
			return t, true
		}
	}
	return nil, false
}

// KillTask cancels the active chain if it matches id. Returns true if killed.
func (c *Coder) KillTask(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.active != nil && c.active.ID == id {
		c.active.cancel()
		return true
	}
	return false
}

// HasActive reports whether a chain is currently running.
func (c *Coder) HasActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active != nil
}

func (c *Coder) loop(ctx context.Context) {
	// Register on the bus. Re-register periodically in case of TTL expiry.
	c.register(ctx)
	lastReg := time.Now()

	for {
		if ctx.Err() != nil {
			return
		}

		if time.Since(lastReg) > 10*time.Minute {
			c.register(ctx)
			lastReg = time.Now()
		}

		envelopes, err := c.bus.Poll(ctx, sessionID, 1, 30*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("coder: poll error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		for _, env := range envelopes {
			var msg CodeTaskMessage
			if err := json.Unmarshal([]byte(env.Message), &msg); err != nil {
				log.Printf("coder: bad message from %s: %v", env.FromSessionID, err)
				continue
			}
			if msg.Type != "code_task" {
				log.Printf("coder: ignoring message type %q", msg.Type)
				continue
			}
			c.sem <- struct{}{}
			go func(msg CodeTaskMessage) {
				defer func() { <-c.sem }()
				c.runChain(ctx, msg)
			}(msg)
		}
	}
}

func (c *Coder) register(ctx context.Context) {
	if _, err := c.bus.Register(ctx, sessionbus.RegisterRequest{
		SessionID: sessionID,
		Provider:  "coder",
		Name:      "Scaffold Coder",
	}); err != nil {
		log.Printf("coder: register on bus: %v", err)
	}
}

func (c *Coder) runChain(ctx context.Context, msg CodeTaskMessage) {
	chain := msg.Chain
	if chain == "" {
		chain = "implement"
	}

	chainDef, ok := builtinChains[chain]
	if !ok {
		log.Printf("coder: unknown chain %q", chain)
		return
	}

	cwd := strings.TrimSpace(msg.CWD)
	if cwd == "" {
		cwd = "/home/mikekey/Builds/scaffold"
	}
	if !c.isAllowedPath(cwd) {
		log.Printf("coder: cwd %q not in allowlist", cwd)
		return
	}

	taskID := uuid.New().String()
	now := time.Now()

	steps := make([]StepInfo, len(chainDef.Steps))
	for i, s := range chainDef.Steps {
		steps[i] = StepInfo{Name: s.Name, Status: "pending"}
	}

	taskCtx, cancel := context.WithCancel(ctx)
	task := &Task{
		ID:        taskID,
		Chain:     chain,
		TaskDesc:  msg.Task,
		CWD:       cwd,
		Status:    "running",
		StartedAt: now,
		Steps:     steps,
		ReplyTo:   msg.ReplyTo,
		cancel:    cancel,
	}

	c.mu.Lock()
	c.active = task
	c.mu.Unlock()

	defer func() {
		cancel()
		ended := time.Now()
		task.EndedAt = &ended

		c.mu.Lock()
		c.active = nil
		c.recent = append([]*Task{task}, c.recent...)
		if len(c.recent) > maxRecent {
			c.recent = c.recent[:maxRecent]
		}
		c.mu.Unlock()
	}()

	runDir, err := initRunDir(runBaseDir, taskID)
	if err != nil {
		log.Printf("coder: init run dir: %v", err)
		return
	}
	task.RunDir = runDir

	writeTaskFile(runDir, msg.Task)
	writeStatusFile(runDir, RunStatus{
		Chain:     chain,
		Task:      msg.Task,
		Status:    "running",
		StartedAt: now,
	})

	baseSkill := c.loadSkill("")

	stepNames := make([]string, len(chainDef.Steps))
	for i, s := range chainDef.Steps {
		stepNames[i] = s.Name
	}
	c.hub.Broadcast("chain_started", map[string]any{
		"task_id": taskID,
		"chain":   chain,
		"task":    msg.Task,
		"steps":   stepNames,
	})

	log.Printf("coder: chain %s started (task_id=%s, chain=%s)", msg.Task, taskID, chain)

	started := time.Now()
	prev := ""

	for i, step := range chainDef.Steps {
		dir := stepDir(runDir, i, step.Name)
		if err := initStepDir(dir); err != nil {
			log.Printf("coder: init step dir: %v", err)
			c.failChain(ctx, task, runDir, step.Name, "failed to create step directory", started, msg.ReplyTo)
			return
		}

		prompt := renderPrompt(step.Prompt, msg.Task, prev, runDir)
		stepSkill := c.loadSkill(step.Name)
		fullPrompt := combineSkillAndPrompt(baseSkill, stepSkill, prompt)
		writePromptFile(dir, fullPrompt)

		task.Steps[i].Status = "running"
		stepStart := time.Now()

		c.hub.Broadcast("step_started", map[string]any{
			"task_id":    taskID,
			"step":       step.Name,
			"step_num":   i + 1,
			"step_total": len(chainDef.Steps),
		})

		log.Printf("coder: step %s/%s started", taskID, step.Name)

		output, runErr := c.runStep(taskCtx, taskID, step.Name, dir, fullPrompt, cwd)
		elapsed := time.Since(stepStart).Seconds()
		task.Steps[i].ElapsedS = elapsed

		if runErr != nil {
			task.Steps[i].Status = "failed"
			errMsg := runErr.Error()
			log.Printf("coder: step %s/%s failed: %v", taskID, step.Name, runErr)
			c.failChain(ctx, task, runDir, step.Name, errMsg, started, msg.ReplyTo)
			return
		}

		task.Steps[i].Status = "done"
		prev = output

		c.hub.Broadcast("step_done", map[string]any{
			"task_id":   taskID,
			"step":      step.Name,
			"elapsed_s": elapsed,
		})
		log.Printf("coder: step %s/%s done (%.1fs)", taskID, step.Name, elapsed)
	}

	elapsed := time.Since(started).Seconds()
	task.Status = "done"
	task.Summary = prev

	ended := time.Now()
	writeStatusFile(runDir, RunStatus{
		Chain:     chain,
		Task:      msg.Task,
		Status:    "done",
		StartedAt: now,
		EndedAt:   &ended,
		Summary:   prev,
	})

	c.hub.Broadcast("chain_done", map[string]any{
		"task_id":   taskID,
		"status":    "done",
		"summary":   prev,
		"elapsed_s": elapsed,
	})

	log.Printf("coder: chain done (task_id=%s, elapsed=%.1fs)", taskID, elapsed)

	c.sendResult(ctx, msg.ReplyTo, CoderResultMessage{
		Type:     "coder_result",
		TaskID:   taskID,
		Chain:    chain,
		Status:   "done",
		Summary:  prev,
		ElapsedS: elapsed,
	})
}

func (c *Coder) failChain(ctx context.Context, task *Task, runDir, failedStep, errMsg string, started time.Time, replyTo string) {
	task.Status = "failed"
	task.FailedStep = failedStep
	task.Error = errMsg

	elapsed := time.Since(started).Seconds()
	now := time.Now()

	writeStatusFile(runDir, RunStatus{
		Chain:      task.Chain,
		Task:       task.TaskDesc,
		Status:     "failed",
		StartedAt:  task.StartedAt,
		EndedAt:    &now,
		FailedStep: failedStep,
		Error:      errMsg,
	})

	c.hub.Broadcast("chain_failed", map[string]any{
		"task_id":     task.ID,
		"failed_step": failedStep,
		"error":       errMsg,
		"elapsed_s":   elapsed,
	})

	c.sendResult(ctx, replyTo, CoderResultMessage{
		Type:       "coder_result",
		TaskID:     task.ID,
		Chain:      task.Chain,
		Status:     "failed",
		FailedStep: failedStep,
		Error:      errMsg,
		ElapsedS:   elapsed,
	})
}

func (c *Coder) sendResult(ctx context.Context, replyTo string, result CoderResultMessage) {
	if replyTo == "" {
		return
	}
	data, err := json.Marshal(result)
	if err != nil {
		return
	}
	if _, err := c.bus.Send(ctx, sessionbus.SendRequest{
		FromSessionID: sessionID,
		ToSessionID:   replyTo,
		Message:       string(data),
	}); err != nil {
		log.Printf("coder: send result to %s: %v", replyTo, err)
	}
}

func (c *Coder) runStep(ctx context.Context, taskID, stepName, dir, prompt, cwd string) (string, error) {
	cmd := exec.Command("claude",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		"-p", prompt,
	)
	cmd.Dir = cwd

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start claude: %w", err)
	}

	// Send SIGTERM on context cancellation
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
	}()

	// Drain stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("coder [%s/%s] stderr: %s", taskID, stepName, scanner.Text())
		}
	}()

	var lastResult string

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		appendStepEvent(dir, line)
		c.parseAndBroadcast(taskID, stepName, line, &lastResult)
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("step cancelled")
		}
		return "", fmt.Errorf("claude exited: %w", err)
	}

	// Prefer output.md written by claude
	if out := readStepOutput(dir); out != "" {
		return out, nil
	}
	// Fall back to result event text
	if lastResult != "" {
		writeStepOutput(dir, lastResult)
		return lastResult, nil
	}
	return "", nil
}

// parseAndBroadcast parses a single claude stream-json line and emits SSE events.
func (c *Coder) parseAndBroadcast(taskID, stepName string, line []byte, lastResult *string) {
	var event struct {
		Type    string          `json:"type"`
		Subtype string          `json:"subtype"`
		Result  string          `json:"result"`
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(line, &event); err != nil {
		return
	}

	switch event.Type {
	case "result":
		if event.Result != "" {
			*lastResult = event.Result
		}
		c.hub.Broadcast("step_event", map[string]any{
			"task_id": taskID,
			"step":    stepName,
			"type":    "result",
			"text":    event.Result,
		})

	case "assistant":
		var msg struct {
			Content []struct {
				Type  string          `json:"type"`
				Text  string          `json:"text"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"content"`
		}
		if err := json.Unmarshal(event.Message, &msg); err != nil {
			return
		}
		for _, item := range msg.Content {
			switch item.Type {
			case "tool_use":
				inputStr := extractToolInput(item.Name, item.Input)
				c.hub.Broadcast("step_event", map[string]any{
					"task_id": taskID,
					"step":    stepName,
					"type":    "tool_use",
					"tool":    item.Name,
					"input":   inputStr,
				})
			case "text":
				text := strings.TrimSpace(item.Text)
				if text != "" {
					c.hub.Broadcast("step_event", map[string]any{
						"task_id": taskID,
						"step":    stepName,
						"type":    "assistant",
						"text":    text,
					})
				}
			}
		}
	}
}

// isAllowedPath checks if cwd is under one of the allowed paths.
func (c *Coder) isAllowedPath(cwd string) bool {
	if len(c.cfg.AllowedPaths) == 0 {
		return false
	}
	for _, allowed := range c.cfg.AllowedPaths {
		allowed = filepath.Clean(allowed)
		cwd = filepath.Clean(cwd)
		if cwd == allowed || strings.HasPrefix(cwd, allowed+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// loadSkill reads a skill file. Pass "" for the base skill.
func (c *Coder) loadSkill(stepName string) string {
	var path string
	if stepName == "" {
		path = c.cfg.SkillPath
	} else if c.cfg.StepSkillsDir != "" {
		path = filepath.Join(c.cfg.StepSkillsDir, stepName+".md")
	}
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// combineSkillAndPrompt prepends skills to the prompt.
func combineSkillAndPrompt(base, step, prompt string) string {
	var parts []string
	if base = strings.TrimSpace(base); base != "" {
		parts = append(parts, base)
	}
	if step = strings.TrimSpace(step); step != "" {
		parts = append(parts, step)
	}
	parts = append(parts, prompt)
	return strings.Join(parts, "\n\n")
}

// renderPrompt replaces template variables in a step prompt.
func renderPrompt(tmpl, task, previous, chainDir string) string {
	r := strings.NewReplacer(
		"{task}", task,
		"{previous}", previous,
		"{chain_dir}", chainDir,
	)
	return r.Replace(tmpl)
}

// extractToolInput pulls a human-readable input string from a tool_use event.
func extractToolInput(toolName string, input json.RawMessage) string {
	switch toolName {
	case "Bash":
		var p struct{ Command string `json:"command"` }
		if err := json.Unmarshal(input, &p); err == nil {
			cmd := p.Command
			if len(cmd) > 80 {
				cmd = cmd[:80] + "..."
			}
			return cmd
		}
	case "Read":
		var p struct{ FilePath string `json:"file_path"` }
		if err := json.Unmarshal(input, &p); err == nil {
			return p.FilePath
		}
	case "Write":
		var p struct{ FilePath string `json:"file_path"` }
		if err := json.Unmarshal(input, &p); err == nil {
			return p.FilePath
		}
	case "Edit", "MultiEdit":
		var p struct{ FilePath string `json:"file_path"` }
		if err := json.Unmarshal(input, &p); err == nil {
			return p.FilePath
		}
	case "Glob":
		var p struct{ Pattern string `json:"pattern"` }
		if err := json.Unmarshal(input, &p); err == nil {
			return p.Pattern
		}
	case "Grep":
		var p struct{ Pattern string `json:"pattern"` }
		if err := json.Unmarshal(input, &p); err == nil {
			return p.Pattern
		}
	case "WebFetch", "WebSearch":
		var p struct {
			URL   string `json:"url"`
			Query string `json:"query"`
		}
		if err := json.Unmarshal(input, &p); err == nil {
			if p.URL != "" {
				return p.URL
			}
			return p.Query
		}
	}

	// Generic fallback
	s := string(input)
	if len(s) > 80 {
		s = s[:80] + "..."
	}
	return s
}
