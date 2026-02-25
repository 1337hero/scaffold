package brain

import "testing"

func TestQuoteGmailQueryValue(t *testing.T) {
	got := quoteGmailQueryValue(`FOLLOW "UP"`)
	want := `"FOLLOW \"UP\""`
	if got != want {
		t.Fatalf("quoted label = %q, want %q", got, want)
	}
}
