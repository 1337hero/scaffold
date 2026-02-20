package brain

import "testing"

func TestParsePrioritizeTasksRawJSONArray(t *testing.T) {
	raw := `[{"title":"Task","micro_steps":["Step 1"],"source_memory_id":"m1","why":"important"}]`
	tasks, err := parsePrioritizeTasks(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Title != "Task" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
}

func TestParsePrioritizeTasksFencedJSON(t *testing.T) {
	raw := "```json\n[{\"title\":\"Task\",\"micro_steps\":[\"Step 1\"],\"source_memory_id\":\"m1\",\"why\":\"important\"}]\n```"
	tasks, err := parsePrioritizeTasks(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Title != "Task" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
}

func TestParsePrioritizeTasksRejectsInvalid(t *testing.T) {
	if _, err := parsePrioritizeTasks("not json"); err == nil {
		t.Fatal("expected parse error")
	}
}
