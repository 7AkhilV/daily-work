package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/7AkhilV/daily-work/internal/activity"
	"github.com/7AkhilV/daily-work/internal/summary"
)

type aiItem struct {
	Text    string `json:"text"`
	Project string `json:"project"`
}

var (
	issueNumRe   = regexp.MustCompile(`(?i)\s*#\d+\b`)
	leadVerbRe   = regexp.MustCompile(`(?i)^(fixed|implemented|updated|refactored|added|resolved|closed)\s+#\d+\s*:?\s*`)
	bulletRe     = regexp.MustCompile(`^[\*\-•]\s+`)
	bracketTagRe = regexp.MustCompile(`(?i)^\[[a-z0-9_-]{1,20}\]\s*`)
	imperativeRe = regexp.MustCompile(`(?i)^(add|enhance|require|refactor|update|fix|implement|create|remove|improve)\b`)
)

// buildPrompt matches the user's Slack daily-update style.
func buildPrompt(input ActivityInput) string {
	projects := uniqueProjects(input.Processed)
	topics := BuildTopics(input.Processed)

	var b strings.Builder
	b.WriteString(`You write a short daily work update for Slack.

A non-technical manager should understand each line in a few seconds.

Output JSON only:
{"items":[{"text":"MAB: ...","project":"MAB"}]}

CRITICAL STYLE:
- ALWAYS start every line with the project short name: "MAB: ..." / "WC: ..." (never [MAB])
- One short line per real product feature (about 8–12 words after the prefix)
- Say what people can do now, not how the code was written
- No JSON, APIs, swagger, endpoints, env vars, imports, nodemon, build scripts, lockfiles, or example payloads
- Do not stack 3 changes into one comma-separated sentence
- Past tense / done: "added", "fixed", "updated"
- Skip chores and docs-only work
- Do not invent work or other people's commits
- If media/signed uploads are in the topics, they MUST get their own line
- If job editing is in the topics, they MUST get their own line

GOOD:
MAB: Media uploads now use secure signed links
MAB: Job posts can be edited after they are created
WC: Chat messages stay when people reconnect

BAD:
Added job editing API endpoint, refactored portfolio links to JSON format, and made job application fields optional
Added nodemon as a development dependency
MAB: Enhanced API documentation with example data
UGQ: Fixed harsh bug
`)
	b.WriteString("Date: " + input.DateLabel + "\n")
	if len(projects) > 0 {
		b.WriteString("Projects: " + strings.Join(projects, ", ") + "\n")
	}
	b.WriteString(fmt.Sprintf("Write about %d topic line(s) (one per topic below).\n\n", max(1, len(topics))))

	if len(topics) == 0 {
		b.WriteString("No activity.\n")
		return b.String()
	}

	b.WriteString("Topics from your COMMITS (summarize EACH as one line):\n")
	for i, t := range topics {
		b.WriteString(fmt.Sprintf("\n%d) Project=%s Topic=%s\n", i+1, t.Project, t.Label))
		for _, d := range t.Details {
			b.WriteString("   - " + d + "\n")
		}
	}
	return b.String()
}

func uniqueProjects(proc *activity.ProcessedActivity) []string {
	if proc == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, c := range proc.Clusters {
		for _, cm := range c.Commits {
			if cm.NoiseHint || cm.IsMerge {
				continue
			}
			add(c.ShortName)
			break
		}
	}
	return out
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
		line := cleanLine(it.Text)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "no merge pull") || strings.Contains(lower, "no activity") {
			continue
		}
		out = append(out, summary.WorkItem{
			Text:    humanizeLine(line),
			Project: strings.TrimSpace(it.Project),
		})
	}
	return out, nil
}
