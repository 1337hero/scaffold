package cortex

import (
	"testing"
	"time"
)

func TestSameBriefMinute(t *testing.T) {
	now := time.Date(2026, time.July, 4, 9, 0, 30, 0, time.UTC)

	if !sameBriefMinute(now, "09:00") {
		t.Fatal("expected schedule to match the current minute")
	}
	if sameBriefMinute(now, "09:01") {
		t.Fatal("did not expect a different minute to match")
	}
	if sameBriefMinute(now, "bad") {
		t.Fatal("did not expect invalid schedule to match")
	}
}

func TestClaimBriefRunOncePerLocalDay(t *testing.T) {
	c := &Cortex{}
	now := time.Date(2026, time.July, 4, 9, 0, 0, 0, time.UTC)

	if !c.claimBriefRun("morning_brief", now) {
		t.Fatal("first claim should succeed")
	}
	if c.claimBriefRun("morning_brief", now) {
		t.Fatal("second claim on same day should not succeed")
	}
	if !c.claimBriefRun("morning_brief", now.AddDate(0, 0, 1)) {
		t.Fatal("next day claim should succeed")
	}
}
