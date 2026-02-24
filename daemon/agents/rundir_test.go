package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDirNoTraversal(t *testing.T) {
	rd, err := initRunDir(t.TempDir(), "test-run")
	if err != nil {
		t.Fatalf("initRunDir failed: %v", err)
	}
	defer rd.Close()

	if err := rd.root.WriteFile("../escape.txt", []byte("nope"), 0o644); err == nil {
		t.Fatal("expected traversal write to fail")
	}
}

func TestInitRunDirRejectsInvalidRunID(t *testing.T) {
	invalid := []string{"", "..", "../escape", "a/b", `a\b`}
	for _, runID := range invalid {
		if _, err := initRunDir(t.TempDir(), runID); err == nil {
			t.Fatalf("expected error for runID %q", runID)
		}
	}
}

func TestRunDirRoundTrip(t *testing.T) {
	rd, err := initRunDir(t.TempDir(), "test-run")
	if err != nil {
		t.Fatalf("initRunDir failed: %v", err)
	}
	defer rd.Close()

	rd.WriteTask("hello")
	got, err := os.ReadFile(filepath.Join(rd.AbsPath, "task.md"))
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("unexpected task content: %q", string(got))
	}

	stepRel, err := rd.InitStepDir(0, "scout")
	if err != nil {
		t.Fatalf("InitStepDir failed: %v", err)
	}
	if strings.Contains(stepRel, "..") {
		t.Fatalf("unexpected traversal in step rel path %q", stepRel)
	}
	rd.WritePrompt(stepRel, "prompt")
	rd.AppendEvent(stepRel, []byte(`{"type":"test"}`))
	rd.WriteOutput(stepRel, "final")
	if out := rd.ReadOutput(stepRel); out != "final" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestInitStepDirSanitizesName(t *testing.T) {
	rd, err := initRunDir(t.TempDir(), "test-run")
	if err != nil {
		t.Fatalf("initRunDir failed: %v", err)
	}
	defer rd.Close()

	stepRel, err := rd.InitStepDir(0, "../../review this?")
	if err != nil {
		t.Fatalf("InitStepDir failed: %v", err)
	}
	if strings.Contains(stepRel, "..") || strings.Contains(stepRel, "/..") {
		t.Fatalf("unexpected traversal in stepRel %q", stepRel)
	}
	if _, err := os.Stat(filepath.Join(rd.AbsPath, stepRel)); err != nil {
		t.Fatalf("expected step dir to exist: %v", err)
	}
}
