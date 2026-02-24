package coder

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RunStatus is written to status.json in the run directory.
type RunStatus struct {
	Chain       string     `json:"chain"`
	Task        string     `json:"task"`
	Status      string     `json:"status"` // running, done, failed, cancelled
	StartedAt   time.Time  `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
	CurrentStep string     `json:"current_step,omitempty"`
	FailedStep  string     `json:"failed_step,omitempty"`
	Summary     string     `json:"summary,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// RunDir is an os.Root-anchored handle to a coder run directory.
// All I/O is kernel-enforced to stay within the run dir.
type RunDir struct {
	root    *os.Root
	AbsPath string
}

func initRunDir(base, runID string) (*RunDir, error) {
	path := filepath.Join(base, runID)
	if err := os.MkdirAll(filepath.Join(path, "steps"), 0o750); err != nil {
		return nil, fmt.Errorf("init run dir: %w", err)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, fmt.Errorf("open run dir root: %w", err)
	}
	return &RunDir{root: root, AbsPath: path}, nil
}

func (r *RunDir) Close() error {
	if r == nil || r.root == nil {
		return nil
	}
	return r.root.Close()
}

func (r *RunDir) stepRelDir(n int, name string) string {
	return filepath.Join("steps", fmt.Sprintf("%d-%s", n+1, name))
}

func (r *RunDir) InitStepDir(n int, name string) (string, error) {
	rel := r.stepRelDir(n, name)
	if err := r.root.MkdirAll(rel, 0o750); err != nil {
		return "", fmt.Errorf("init step dir: %w", err)
	}
	return rel, nil
}

func (r *RunDir) WriteTask(task string) {
	_ = r.root.WriteFile("task.md", []byte(task), 0o640)
}

func (r *RunDir) WriteStatus(status RunStatus) {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return
	}
	_ = r.root.WriteFile("status.json", data, 0o640)
}

func (r *RunDir) WritePrompt(stepRel, prompt string) {
	_ = r.root.WriteFile(filepath.Join(stepRel, "prompt.md"), []byte(prompt), 0o640)
}

func (r *RunDir) AppendEvent(stepRel string, data []byte) {
	f, err := r.root.OpenFile(filepath.Join(stepRel, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}

func (r *RunDir) ReadOutput(stepRel string) string {
	data, err := r.root.ReadFile(filepath.Join(stepRel, "output.md"))
	if err != nil || len(data) == 0 {
		return ""
	}
	return string(data)
}

func (r *RunDir) WriteOutput(stepRel, content string) {
	_ = r.root.WriteFile(filepath.Join(stepRel, "output.md"), []byte(content), 0o640)
}
