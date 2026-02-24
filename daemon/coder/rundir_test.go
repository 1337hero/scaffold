package coder

import (
	"os"
	"path/filepath"
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
	rd.WritePrompt(stepRel, "prompt")
	rd.AppendEvent(stepRel, []byte(`{"type":"test"}`))
	rd.WriteOutput(stepRel, "final")
	if out := rd.ReadOutput(stepRel); out != "final" {
		t.Fatalf("unexpected output: %q", out)
	}
}
