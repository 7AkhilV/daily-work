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
	b.WriteString(`You write a developer's daily Slack work update.

Output JSON only:
{"items":[{"text":"MAB: ...","project":"MAB"}]}

CRITICAL STYLE:
- ALWAYS start every line with the project short name: "MAB: ..." / "WC: ..." (never [MAB])
`)
	b.WriteString(`- COMMITS are the source of truth — summarize from commit messages AND changed files, not PR titles
- One line per FEATURE / TOPIC (merge related commits into ONE line)
- Infer the product feature from code files (handlers, services, APIs)
- Prefer real features: signed URL uploads, job post editing APIs, bug fixes
- Do NOT write lines about swagger/openapi/example request-response payloads when code files show a real feature
- Do NOT write tiny lines like "title added" and "title updated" separately
- Past-tense / done wording: "done", "fixed", "added", "updated" — not "Add", "Enhance", "Require"
- No issue numbers (#509), no markdown, no commentary
- Skip merges/prettier/lockfile/docs-only/nodemon/build-script noise
- Do not invent work
- Do not include work from other people's repositories/commits
- If signed/media uploads are in the topics, they MUST get their own line

GOOD (one feature = one line):
WC: Scheduled inspect live flow with title support and admin panel integration
MAB: Media uploads converted to signed URL uploads
MAB: Job post editing APIs added

BAD (never):
WC: schedule inspect creation title added
MAB: Enhanced API documentation with example data...
MAB: Added example data structures and request/response formats...
UGQ: Fixed harsh bug
QuestAndGames: Fixed harsh bug
WC: Add scheduled inspect live flow...
WC: [WC] Chat module fix
CM: Refactor chat service...

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
