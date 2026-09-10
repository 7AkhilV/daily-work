package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/7AkhilV/daily-work/internal/activity"
	"github.com/7AkhilV/daily-work/internal/summary"
)

type aiItem struct {
	Text    string   `json:"text"`
	Project string   `json:"project"`
	Sources []string `json:"sources"`
}

// buildPrompt keeps context small for low-RAM local models.
func buildPrompt(input ActivityInput) string {
	var b strings.Builder
	b.WriteString(`Write a concise daily work update as JSON only:
{"items":[{"text":"Project: past-tense summary","project":"Project"}]}

Rules: group related commits; past tense; project prefix; no invention; skip merges/prettier/lockfiles; few bullets.

`)
	b.WriteString("Date: " + input.DateLabel + "\n")
	if input.Processed == nil {
		b.WriteString("No activity.\n")
		return b.String()
	}

	for i, c := range input.Processed.Clusters {
		if i >= 12 {
			b.WriteString("...(more clusters omitted)\n")
			break
		}
		b.WriteString(fmt.Sprintf("\n[%s]\n", c.ShortName))
		for j, cm := range c.Commits {
			if j >= 6 {
				break
			}
			flag := ""
			if cm.NoiseHint {
				flag = " [noise?]"
			}
			b.WriteString(fmt.Sprintf("- %s%s\n", cm.Message, flag))
			// File names only — no patches (saves RAM/tokens on small Macs)
			n := 0
			for _, f := range cm.Files {
				if n >= 5 {
					break
				}
				b.WriteString(fmt.Sprintf("  %s\n", f.Filename))
				n++
			}
		}
	}

	for i, pr := range input.Processed.PRs {
		if i == 0 {
			b.WriteString("\nPRs:\n")
		}
		if i >= 8 {
			break
		}
		b.WriteString(fmt.Sprintf("- %s: #%d %s\n", activity.ShortName(pr.RepoName), pr.Number, pr.Title))
	}
	return b.String()
}

func parseWorkItems(text string) ([]summary.WorkItem, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}

	var envelope struct {
		Items []aiItem `json:"items"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		var arr []aiItem
		if err2 := json.Unmarshal([]byte(text), &arr); err2 != nil {
			return nil, fmt.Errorf("parse AI JSON: %w", err)
		}
		envelope.Items = arr
	}

	var out []summary.WorkItem
	for _, it := range envelope.Items {
		line := strings.TrimSpace(it.Text)
		line = strings.TrimPrefix(line, "- ")
		if line == "" {
			continue
		}
		out = append(out, summary.WorkItem{
			Text:      line,
			Project:   it.Project,
			SourceIDs: it.Sources,
		})
	}
	return out, nil
}

// FallbackFromActivity builds crude bullets when AI is unavailable.
func FallbackFromActivity(proc *activity.ProcessedActivity) []summary.WorkItem {
	if proc == nil {
		return nil
	}
	var items []summary.WorkItem
	for _, c := range proc.Clusters {
		msgs := make([]string, 0, len(c.Commits))
		ids := make([]string, 0, len(c.Commits))
		for _, cm := range c.Commits {
			if cm.NoiseHint {
				continue
			}
			msgs = append(msgs, cm.Message)
			ids = append(ids, cm.SHA)
		}
		if len(msgs) == 0 && len(c.Commits) > 0 {
			msgs = append(msgs, c.Commits[0].Message)
			ids = append(ids, c.Commits[0].SHA)
		}
		if len(msgs) == 0 {
			continue
		}
		desc := msgs[0]
		if len(msgs) > 1 {
			desc = msgs[0] + " (+ related changes)"
		}
		items = append(items, summary.WorkItem{
			Text:      fmt.Sprintf("%s: %s", c.ShortName, capitalizeFirst(desc)),
			Project:   c.ShortName,
			SourceIDs: ids,
		})
	}

	for _, pr := range proc.PRs {
		short := activity.ShortName(pr.RepoName)
		title := strings.TrimSpace(pr.Title)
		if title == "" {
			continue
		}
		items = append(items, summary.WorkItem{
			Text:    fmt.Sprintf("%s: %s", short, capitalizeFirst(title)),
			Project: short,
		})
	}
	return dedupeItems(items)
}

func dedupeItems(items []summary.WorkItem) []summary.WorkItem {
	seen := map[string]bool{}
	var out []summary.WorkItem
	for _, it := range items {
		key := strings.ToLower(strings.TrimSpace(it.Text))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, it)
	}
	return out
}

func capitalizeFirst(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] = r[0] - 32
	}
	return string(r)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
