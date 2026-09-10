package summary

import (
	"strings"
	"testing"
	"time"
)

func TestFormatSummary(t *testing.T) {
	day := time.Date(2026, 9, 10, 0, 0, 0, 0, time.Local)
	out := FormatSummary(day, []WorkItem{
		{Text: "Suzhi: Gallery API"},
		{Text: "- WC: Fixed chat"},
	})
	if !strings.HasPrefix(out, "10-09-2026\n") {
		t.Fatalf("unexpected date header: %q", out)
	}
	if !strings.Contains(out, "- Suzhi: Gallery API\n") {
		t.Fatalf("missing first bullet: %q", out)
	}
	if !strings.Contains(out, "- WC: Fixed chat\n") {
		t.Fatalf("missing second bullet: %q", out)
	}
}

func TestMergePreservingManual(t *testing.T) {
	existing := []WorkItem{
		{Text: "AI old", Manual: false},
		{Text: "Manual task", Manual: true},
		{Text: "Edited task", Edited: true},
	}
	generated := []WorkItem{{Text: "AI new"}}
	merged := MergePreservingManual(existing, generated)
	if len(merged) != 3 {
		t.Fatalf("len=%d want 3", len(merged))
	}
	if merged[0].Text != "AI new" {
		t.Fatalf("expected generated first, got %q", merged[0].Text)
	}
}
