package agents

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"

	"scaffold/sessionbus"
)

const (
	sessionID  = "scaffold-worker"
	runBaseDir = "/tmp/scaffold-worker"
	maxRecent  = 20
)

// Config holds agent runner configuration.
type Config struct {
	AllowedPaths  []string
	DefaultCWD    string
	MaxConcurrent int
	PromptsDir    string
	SkillPath     string // path to coder-skill.md
	StepSkillsDir string // path to coder-skills/ dir
	PiBinary      string // path to pi binary, default "pi"
	Provider      string // LLM provider for pi (anthropic, openai, etc.)
	Model         string // model ID for pi
	APIKeyEnv     string // env var name holding the API key
	Chains        map[string]Chain
}

// Coder listens for code_task messages on the session bus and runs chains.
type Coder struct {
	bus *sessionbus.Bus
	hub *SSEHub
	sem chan struct{}
	cfg Config

	mu     sync.RWMutex
	active map[string]*Task
	recent []*Task
}

// Task tracks a running or completed chain.
type Task struct {
	ID         string     `json:"id"`
	Chain      string     `json:"chain"`
	TaskDesc   string     `json:"task"`
	CWD        string     `json:"cwd"`
	Status     string     `json:"status"` // running, done, failed, cancelled
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	Steps      []StepInfo `json:"steps"`
	Summary    string     `json:"summary,omitempty"`
	FailedStep string     `json:"failed_step,omitempty"`
	Error      string     `json:"error,omitempty"`
	RunDir     string     `json:"run_dir"`
	ReplyTo    string     `json:"-"`

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
	maxC := cfg.MaxConcurrent
	if maxC < 1 {
		maxC = 1
	}
	return &Coder{
		bus:    bus,
		hub:    newSSEHub(),
		sem:    make(chan struct{}, maxC),
		cfg:    cfg,
		active: make(map[string]*Task),
	}
}

// Hub returns the SSE hub for API handler use.
func (c *Coder) Hub() *SSEHub {
	return c.hub
}

// ChainNames returns sorted chain names for API consumers.
func (c *Coder) ChainNames() []string {
	names := make([]string, 0, len(c.cfg.Chains))
	for k := range c.cfg.Chains {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// ValidateChain reports whether chain name exists in config.
func (c *Coder) ValidateChain(name string) bool {
	_, ok := c.cfg.Chains[name]
	return ok
}

// IsAllowedPath reports whether cwd is under an allowed path.
func (c *Coder) IsAllowedPath(path string) bool {
	return c.isAllowedPath(path)
}

// DefaultCWD returns the configured default working directory.
func (c *Coder) DefaultCWD() string {
	if c.cfg.DefaultCWD != "" {
		return c.cfg.DefaultCWD
	}
	if len(c.cfg.AllowedPaths) > 0 {
		return c.cfg.AllowedPaths[0]
	}
	return ""
}

// Start begins the bus consumer loop in a goroutine.
func (c *Coder) Start(ctx context.Context) {
	go c.loop(ctx)
}

// ListTasks returns active + recent tasks (active first). Always returns a non-nil slice.
func (c *Coder) ListTasks() []*Task {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]*Task, 0, len(c.active)+len(c.recent))
	for _, t := range c.active {
		out = append(out, t)
	}
	out = append(out, c.recent...)
	return out
}

// GetTask looks up a task by ID.
func (c *Coder) GetTask(id string) (*Task, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if t, ok := c.active[id]; ok {
		return t, true
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

	if t, ok := c.active[id]; ok {
		t.cancel()
		return true
	}
	return false
}

// HasActive reports whether a chain is currently running.
func (c *Coder) HasActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.active) > 0
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
			log.Printf("agents: poll error: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		for _, env := range envelopes {
			var msg CodeTaskMessage
			if err := json.Unmarshal([]byte(env.Message), &msg); err != nil {
				log.Printf("agents: bad message from %s: %v", env.FromSessionID, err)
				continue
			}
			if msg.Type != "code_task" {
				log.Printf("agents: ignoring message type %q", msg.Type)
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
		Provider:  "agents",
		Name:      "Scaffold Worker",
	}); err != nil {
		log.Printf("agents: register on bus: %v", err)
	}
}

func (c *Coder) runChain(ctx context.Context, msg CodeTaskMessage) {
	chain := msg.Chain
	if chain == "" {
		chain = "implement"
	}

	chainDef, ok := c.cfg.Chains[chain]
	if !ok {
		log.Printf("agents: unknown chain %q", chain)
		c.sendResult(ctx, msg.ReplyTo, CoderResultMessage{
			Type:   "coder_result",
			Chain:  chain,
			Status: "failed",
			Error:  fmt.Sprintf("unknown chain %q", chain),
		})
		return
	}

	cwd := strings.TrimSpace(msg.CWD)
	if cwd == "" {
		cwd = c.cfg.DefaultCWD
	}
	if cwd == "" && len(c.cfg.AllowedPaths) > 0 {
		cwd = c.cfg.AllowedPaths[0]
	}
	if cwd == "" {
		wd, err := os.Getwd()
		if err != nil {
			log.Printf("agents: resolve cwd: %v", err)
			return
		}
		cwd = wd
	}
	if !c.isAllowedPath(cwd) {
		log.Printf("agents: cwd %q not in allowlist", cwd)
		c.sendResult(ctx, msg.ReplyTo, CoderResultMessage{
			Type:   "coder_result",
			Chain:  chain,
			Status: "failed",
			Error:  fmt.Sprintf("cwd %q not in allowlist", cwd),
		})
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
	c.active[taskID] = task
	c.mu.Unlock()

	defer func() {
		cancel()
		ended := time.Now()
		task.EndedAt = &ended

		c.mu.Lock()
		delete(c.active, taskID)
		c.recent = append([]*Task{task}, c.recent...)
		if len(c.recent) > maxRecent {
			c.recent = c.recent[:maxRecent]
		}
		c.mu.Unlock()
	}()

	runDir, err := initRunDir(runBaseDir, taskID)
	if err != nil {
		log.Printf("agents: init run dir: %v", err)
		return
	}
	defer func() {
		if err := runDir.Close(); err != nil {
			log.Printf("agents: close run dir root: %v", err)
		}
	}()
	task.RunDir = runDir.AbsPath

	runDir.WriteTask(msg.Task)
	runDir.WriteStatus(RunStatus{
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

	log.Printf("agents: chain %s started (task_id=%s, chain=%s)", msg.Task, taskID, chain)

	started := time.Now()
	prev := ""

	for i, step := range chainDef.Steps {
		stepRel, err := runDir.InitStepDir(i, step.Name)
		if err != nil {
			log.Printf("agents: init step dir: %v", err)
			c.failChain(ctx, task, runDir, step.Name, "failed to create step directory", started, msg.ReplyTo)
			return
		}

		promptTmpl, err := loadPromptFile(c.cfg.PromptsDir, step.PromptFile)
		if err != nil {
			log.Printf("agents: load prompt %s: %v", step.PromptFile, err)
			c.failChain(ctx, task, runDir, step.Name, fmt.Sprintf("load prompt: %v", err), started, msg.ReplyTo)
			return
		}
		prompt := renderPrompt(promptTmpl, msg.Task, prev, runDir.AbsPath)
		stepSkill := c.loadSkill(step.Name)
		fullPrompt := combineSkillAndPrompt(baseSkill, stepSkill, prompt)
		runDir.WritePrompt(stepRel, fullPrompt)

		task.Steps[i].Status = "running"
		stepStart := time.Now()

		c.hub.Broadcast("step_started", map[string]any{
			"task_id":    taskID,
			"step":       step.Name,
			"step_num":   i + 1,
			"step_total": len(chainDef.Steps),
		})

		log.Printf("agents: step %s/%s started", taskID, step.Name)

		output, runErr := c.runStep(taskCtx, taskID, step.Name, runDir, stepRel, fullPrompt, cwd, step.Tools)
		elapsed := time.Since(stepStart).Seconds()
		task.Steps[i].ElapsedS = elapsed

		if runErr != nil {
			task.Steps[i].Status = "failed"
			errMsg := runErr.Error()
			log.Printf("agents: step %s/%s failed: %v", taskID, step.Name, runErr)
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
		log.Printf("agents: step %s/%s done (%.1fs)", taskID, step.Name, elapsed)
	}

	elapsed := time.Since(started).Seconds()
	task.Status = "done"
	task.Summary = prev

	ended := time.Now()
	runDir.WriteStatus(RunStatus{
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

	log.Printf("agents: chain done (task_id=%s, elapsed=%.1fs)", taskID, elapsed)

	c.sendResult(ctx, msg.ReplyTo, CoderResultMessage{
		Type:     "coder_result",
		TaskID:   taskID,
		Chain:    chain,
		Status:   "done",
		Summary:  prev,
		ElapsedS: elapsed,
	})
}

func (c *Coder) failChain(ctx context.Context, task *Task, runDir *RunDir, failedStep, errMsg string, started time.Time, replyTo string) {
	task.Status = "failed"
	task.FailedStep = failedStep
	task.Error = errMsg

	elapsed := time.Since(started).Seconds()
	now := time.Now()

	runDir.WriteStatus(RunStatus{
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
		log.Printf("agents: send result to %s: %v", replyTo, err)
	}
}

func (c *Coder) runStep(ctx context.Context, taskID, stepName string, runDir *RunDir, stepRel, prompt, cwd string, tools []string) (string, error) {
	piBin := c.cfg.PiBinary
	if piBin == "" {
		piBin = "pi"
	}

	toolSet := "read,write,edit,bash,grep,find,ls"
	if len(tools) > 0 {
		toolSet = strings.Join(tools, ",")
	}

	args := []string{
		"--mode", "rpc",
		"--provider", c.cfg.Provider,
		"--model", c.cfg.Model,
		"--tools", toolSet,
		"--no-session",
	}

	cmd := exec.Command(piBin, args...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start pi: %w", err)
	}

	// Send prompt as JSON-RPC
	promptMsg := map[string]string{
		"id":      "step",
		"type":    "prompt",
		"message": prompt,
	}
	if err := json.NewEncoder(stdin).Encode(promptMsg); err != nil {
		return "", fmt.Errorf("write prompt to pi: %w", err)
	}

	// Context cancellation → abort then sigterm
	go func() {
		<-ctx.Done()
		// Try graceful abort
		_ = json.NewEncoder(stdin).Encode(map[string]string{"type": "abort", "id": "cancel"})
		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
		}
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
	agentDone := false

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		runDir.AppendEvent(stepRel, line)
		c.parsePiEvent(taskID, stepName, line, &lastResult)

		// pi RPC is long-lived — break when agent finishes
		var ev struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(line, &ev) == nil && ev.Type == "agent_end" {
			agentDone = true
			break
		}
	}

	// Close stdin so pi exits cleanly
	stdin.Close()
	_ = agentDone

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("step cancelled")
		}
		return "", fmt.Errorf("pi exited: %w", err)
	}

	// Prefer output.md written by agent
	if out := runDir.ReadOutput(stepRel); out != "" {
		return out, nil
	}
	if lastResult != "" {
		runDir.WriteOutput(stepRel, lastResult)
		return lastResult, nil
	}
	return "", nil
}

// parsePiEvent parses a single pi RPC event line and emits SSE events.
func (c *Coder) parsePiEvent(taskID, stepName string, line []byte, lastResult *string) {
	var event struct {
		Type                  string          `json:"type"`
		ToolName              string          `json:"toolName"`
		Args                  json.RawMessage `json:"args"`
		Result                json.RawMessage `json:"result"`
		AssistantMessageEvent json.RawMessage `json:"assistantMessageEvent"`
		Messages              json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(line, &event); err != nil {
		return
	}

	switch event.Type {
	case "tool_execution_start":
		inputStr := extractToolInput(event.ToolName, event.Args)
		c.hub.Broadcast("step_event", map[string]any{
			"task_id": taskID,
			"step":    stepName,
			"type":    "tool_use",
			"tool":    event.ToolName,
			"input":   inputStr,
		})

	case "tool_execution_end":
		resultStr := extractToolResult(event.ToolName, event.Result)
		c.hub.Broadcast("step_event", map[string]any{
			"task_id": taskID,
			"step":    stepName,
			"type":    "tool_result",
			"tool":    event.ToolName,
			"result":  resultStr,
		})

	case "message_update":
		if event.AssistantMessageEvent == nil {
			return
		}
		var ame struct {
			Type  string `json:"type"`
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(event.AssistantMessageEvent, &ame); err != nil {
			return
		}
		if ame.Type == "text_delta" && strings.TrimSpace(ame.Delta) != "" {
			c.hub.Broadcast("step_event", map[string]any{
				"task_id": taskID,
				"step":    stepName,
				"type":    "assistant",
				"text":    ame.Delta,
			})
		}

	case "agent_end":
		// Extract final assistant text from messages array
		if event.Messages == nil {
			return
		}
		var msgs []struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(event.Messages, &msgs); err != nil {
			return
		}
		// Walk backwards to find last assistant text
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Role == "assistant" {
				for _, c := range msgs[i].Content {
					if c.Type == "text" && c.Text != "" {
						*lastResult = c.Text
						return
					}
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
	resolvedCWD, err := canonicalPath(cwd)
	if err != nil {
		return false
	}
	for _, allowed := range c.cfg.AllowedPaths {
		resolvedAllowed, err := canonicalPath(allowed)
		if err != nil {
			continue
		}
		if resolvedCWD == resolvedAllowed || strings.HasPrefix(resolvedCWD, resolvedAllowed+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
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

// extractToolInput pulls a human-readable input string from a tool args object.
// Handles both pi tool names (lowercase) and Claude tool names (PascalCase).
func extractToolInput(toolName string, input json.RawMessage) string {
	switch toolName {
	case "bash", "Bash":
		var p struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(input, &p); err == nil {
			cmd := p.Command
			if len(cmd) > 80 {
				cmd = cmd[:80] + "..."
			}
			return cmd
		}
	case "read", "Read":
		var p struct {
			Path     string `json:"path"`
			FilePath string `json:"file_path"`
		}
		if err := json.Unmarshal(input, &p); err == nil {
			if p.Path != "" {
				return p.Path
			}
			return p.FilePath
		}
	case "write", "Write":
		var p struct {
			Path     string `json:"path"`
			FilePath string `json:"file_path"`
		}
		if err := json.Unmarshal(input, &p); err == nil {
			if p.Path != "" {
				return p.Path
			}
			return p.FilePath
		}
	case "edit", "Edit", "MultiEdit":
		var p struct {
			Path     string `json:"path"`
			FilePath string `json:"file_path"`
		}
		if err := json.Unmarshal(input, &p); err == nil {
			if p.Path != "" {
				return p.Path
			}
			return p.FilePath
		}
	case "grep", "Grep":
		var p struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal(input, &p); err == nil {
			return p.Pattern
		}
	case "find", "Glob":
		var p struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal(input, &p); err == nil {
			return p.Pattern
		}
	case "ls":
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(input, &p); err == nil {
			return p.Path
		}
	}

	// Generic fallback
	s := string(input)
	if len(s) > 80 {
		s = s[:80] + "..."
	}
	return s
}

// extractToolResult pulls a human-readable result string from a tool result object.
func extractToolResult(toolName string, result json.RawMessage) string {
	if result == nil {
		return ""
	}

	var str string
	if err := json.Unmarshal(result, &str); err == nil {
		if len(str) > 200 {
			str = str[:200] + "..."
		}
		return str
	}

	var obj struct {
		Output  string `json:"output"`
		Content string `json:"content"`
		Stdout  string `json:"stdout"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(result, &obj); err == nil {
		out := obj.Output
		if out == "" {
			out = obj.Content
		}
		if out == "" {
			out = obj.Stdout
		}
		if out == "" {
			out = obj.Error
		}
		if len(out) > 200 {
			out = out[:200] + "..."
		}
		if out != "" {
			return out
		}
	}

	s := string(result)
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

// ReadStepEvents reads events.jsonl for a given task and step number.
func (c *Coder) ReadStepEvents(taskID, stepNum string) ([]map[string]any, error) {
	task, ok := c.GetTask(taskID)
	if !ok {
		return nil, fmt.Errorf("task not found")
	}
	if task.RunDir == "" {
		return nil, fmt.Errorf("no run directory")
	}

	stepsDir := filepath.Join(task.RunDir, "steps")
	entries, err := os.ReadDir(stepsDir)
	if err != nil {
		return nil, fmt.Errorf("read steps dir: %w", err)
	}

	var stepDir string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), stepNum+"-") {
			stepDir = filepath.Join(stepsDir, e.Name())
			break
		}
	}
	if stepDir == "" {
		return nil, fmt.Errorf("step %s not found", stepNum)
	}

	eventsPath := filepath.Join(stepDir, "events.jsonl")
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}

	events := make([]map[string]any, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		parsed := parseStoredEvent(line)
		if parsed != nil {
			events = append(events, parsed)
		}
	}
	return events, nil
}

// parseStoredEvent converts a raw pi RPC event line into the same shape as SSE step_events.
func parseStoredEvent(line string) map[string]any {
	var event struct {
		Type                  string          `json:"type"`
		ToolName              string          `json:"toolName"`
		Args                  json.RawMessage `json:"args"`
		Result                json.RawMessage `json:"result"`
		AssistantMessageEvent json.RawMessage `json:"assistantMessageEvent"`
	}
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return nil
	}

	switch event.Type {
	case "tool_execution_start":
		inputStr := extractToolInput(event.ToolName, event.Args)
		return map[string]any{
			"type":  "tool_use",
			"tool":  event.ToolName,
			"input": inputStr,
		}
	case "tool_execution_end":
		resultStr := extractToolResult(event.ToolName, event.Result)
		return map[string]any{
			"type":   "tool_result",
			"tool":   event.ToolName,
			"result": resultStr,
		}
	case "message_update":
		if event.AssistantMessageEvent == nil {
			return nil
		}
		var ame struct {
			Type  string `json:"type"`
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(event.AssistantMessageEvent, &ame); err != nil {
			return nil
		}
		if ame.Type == "text_delta" && strings.TrimSpace(ame.Delta) != "" {
			return map[string]any{
				"type": "assistant",
				"text": ame.Delta,
			}
		}
	}
	return nil
}
