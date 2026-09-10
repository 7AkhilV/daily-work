package summary

import (
	"fmt"
	"strings"
	"time"
)

// WorkItem is one line in the daily summary.
type WorkItem struct {
	Text      string   `json:"text"`
	Project   string   `json:"project,omitempty"`
	Manual    bool     `json:"manual,omitempty"`
	Edited    bool     `json:"edited,omitempty"`
	SourceIDs []string `json:"source_ids,omitempty"`
}

// FormatDate returns DD-MM-YYYY for display.
func FormatDate(t time.Time) string {
	return t.Format("02-01-2006")
}

// FormatSummary builds the clipboard text matching the user's Slack style
// (date, blank line, plain lines — no leading "- ").
func FormatSummary(date time.Time, items []WorkItem) string {
	var b strings.Builder
	b.WriteString(FormatDate(date))
	b.WriteString("\n\n")
	for _, item := range items {
		line := strings.TrimSpace(item.Text)
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		if line == "" {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// DisplayLine returns the line for the interactive UI.
func (w WorkItem) DisplayLine() string {
	line := strings.TrimSpace(w.Text)
	line = strings.TrimPrefix(line, "- ")
	line = strings.TrimPrefix(line, "* ")
	return line
}

// ParseTaskInput normalizes user-entered task text.
func ParseTaskInput(input string) string {
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, "- ")
	input = strings.TrimPrefix(input, "* ")
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	return input
}

// MergePreservingManual keeps manual/edited items and replaces AI ones.
func MergePreservingManual(existing, generated []WorkItem) []WorkItem {
	var preserved []WorkItem
	for _, item := range existing {
		if item.Manual || item.Edited {
			preserved = append(preserved, item)
		}
	}
	return append(generated, preserved...)
}

func ValidateNonEmpty(items []WorkItem) error {
	for _, item := range items {
		if strings.TrimSpace(item.Text) != "" {
			return nil
		}
	}
	return fmt.Errorf("no work items")
}
