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

func initRunDir(base, runID string) (string, error) {
	path := filepath.Join(base, runID)
	if err := os.MkdirAll(filepath.Join(path, "steps"), 0o750); err != nil {
		return "", fmt.Errorf("init run dir: %w", err)
	}
	return path, nil
}

func stepDir(runDir string, n int, name string) string {
	return filepath.Join(runDir, "steps", fmt.Sprintf("%d-%s", n+1, name))
}

func writeTaskFile(runDir, task string) {
	_ = os.WriteFile(filepath.Join(runDir, "task.md"), []byte(task), 0o640)
}

func writeStatusFile(runDir string, status RunStatus) {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(runDir, "status.json"), data, 0o640)
}

func initStepDir(dir string) error {
	return os.MkdirAll(dir, 0o750)
}

func writePromptFile(dir, prompt string) {
	_ = os.WriteFile(filepath.Join(dir, "prompt.md"), []byte(prompt), 0o640)
}

func appendStepEvent(dir string, data []byte) {
	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}

func readStepOutput(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "output.md"))
	if err != nil || len(data) == 0 {
		return ""
	}
	return string(data)
}

func writeStepOutput(dir, content string) {
	_ = os.WriteFile(filepath.Join(dir, "output.md"), []byte(content), 0o640)
}
