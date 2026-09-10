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
)

// buildPrompt matches the user's Slack daily-update style.
func buildPrompt(input ActivityInput) string {
	projects := uniqueProjects(input.Processed)
	multi := len(projects) > 1

	var b strings.Builder
	b.WriteString(`You write a developer's daily Slack work update.

Output JSON only:
{"items":[{"text":"...","project":"WC"}]}

STYLE (follow exactly):
`)
	if multi {
		b.WriteString(`- Multiple projects today → EVERY line MUST start with "Project: " (e.g. "WC: ...", "Suzhi: ...")
`)
	} else {
		b.WriteString(`- Only one project today → do NOT put a project prefix; just the task text
`)
	}
	b.WriteString(`- Short, plain, human lines (like speaking to teammates)
- One line per distinct task. A busy day usually has SEVERAL lines (often 3–8), not one.
- Only merge commits/PRs when they are clearly the SAME feature
- Never use square brackets like [WC] — write "WC: ..." only when a project prefix is required
- No issue/PR numbers ever (never write #509, #10, "Fixed #506:", etc.)
- No markdown, no bullets (* or -), no extra commentary
- Skip merges, prettier, lockfile-only, noise
- Prefer wording like: "done", "fixed", "added", "updated"
- Do not invent work
- Cover the different PRs/commits below — do not drop most of them into a single line

Examples of GOOD output for a multi-project day:
{"items":[
  {"text":"Suzhi: Event gallery api done for both FE and admin panel","project":"Suzhi"},
  {"text":"WC: schedule inspect creation title added","project":"WC"},
  {"text":"WC: Chat module bug fixed for chat persistence","project":"WC"},
  {"text":"WC: Impacts bug fixed","project":"WC"}
]}

Examples of BAD lines (never do this):
WC: [WC] Chat module fix
* Fixed #509: Added scheduled inspect live flow
Implemented #508: Enhanced admin metrics
No merge pull requests were merged today.
(only one item when there were many different PRs)

`)
	b.WriteString("Date: " + input.DateLabel + "\n")
	if len(projects) > 0 {
		b.WriteString("Projects today: " + strings.Join(projects, ", ") + "\n")
		if multi {
			b.WriteString("Use these exact project labels as prefixes (WC: ..., Suzhi: ...). Never [WC].\n")
		} else {
			b.WriteString("Single project — omit the prefix in text.\n")
		}
	}
	if input.Processed == nil {
		b.WriteString("No activity.\n")
		return b.String()
	}

	// Hint how many lines we expect so the small model does not collapse everything.
	approx := approxTaskCount(input.Processed)
	b.WriteString(fmt.Sprintf("Aim for about %d separate items (not fewer than %d unless truly duplicates).\n", approx, max(1, approx/2)))

	for i, c := range input.Processed.Clusters {
		if i >= 12 {
			b.WriteString("...(more omitted)\n")
			break
		}
		b.WriteString(fmt.Sprintf("\nProject %s (repo %s)\n", c.ShortName, c.RepoName))
		for j, cm := range c.Commits {
			if j >= 6 {
				break
			}
			flag := ""
			if cm.NoiseHint {
				flag = " [noise?]"
			}
			msg := issueNumRe.ReplaceAllString(cm.Message, "")
			msg = strings.TrimSpace(msg)
			b.WriteString(fmt.Sprintf("- %s%s\n", msg, flag))
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
			b.WriteString("\nPRs (each may be its own line unless clearly same feature — never the number):\n")
		}
		if i >= 8 {
			break
		}
		short := activity.ShortName(pr.RepoName)
		title := issueNumRe.ReplaceAllString(pr.Title, "")
		title = strings.TrimSpace(title)
		b.WriteString(fmt.Sprintf("- Project %s — %s\n", short, title))
	}
	return b.String()
}

func approxTaskCount(proc *activity.ProcessedActivity) int {
	if proc == nil {
		return 1
	}
	n := 0
	for _, c := range proc.Clusters {
		if c.NoiseHint && len(c.Commits) == 1 {
			continue
		}
		n++
	}
	n += len(proc.PRs)
	if n < 1 {
		n = 1
	}
	if n > 8 {
		n = 8
	}
	return n
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
		add(c.ShortName)
	}
	for _, pr := range proc.PRs {
		add(activity.ShortName(pr.RepoName))
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
		// Drop junk commentary lines the model sometimes adds
		lower := strings.ToLower(line)
		if strings.Contains(lower, "no merge pull") || strings.Contains(lower, "no activity") {
			continue
		}
		out = append(out, summary.WorkItem{
			Text:    line,
			Project: strings.TrimSpace(it.Project),
		})
	}
	return out, nil
}

func cleanLine(s string) string {
	s = strings.TrimSpace(s)
	s = bulletRe.ReplaceAllString(s, "")
	s = leadVerbRe.ReplaceAllString(s, "")
	s = issueNumRe.ReplaceAllString(s, "")
	// Remove [WC] / [Suzhi] style tags the model copies from bad prompts
	for bracketTagRe.MatchString(s) {
		s = strings.TrimSpace(bracketTagRe.ReplaceAllString(s, ""))
	}
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, ":")
	s = strings.TrimSpace(s)
	return s
}

// StyleItems applies project-prefix rules to match the user's reporting style.
func StyleItems(items []summary.WorkItem, proc *activity.ProcessedActivity) []summary.WorkItem {
	projects := uniqueProjects(proc)
	multi := len(projects) > 1
	var out []summary.WorkItem
	for _, it := range items {
		line := cleanLine(it.Text)
		if line == "" {
			continue
		}
		proj := strings.TrimSpace(it.Project)
		proj = bracketTagRe.ReplaceAllString(proj, "")
		proj = strings.Trim(proj, "[] ")
		if proj == "" {
			proj = detectPrefix(line)
		}
		// Keep stripping bracket tags + project prefixes until stable
		for i := 0; i < 3; i++ {
			next := cleanLine(line)
			next = stripKnownPrefix(next)
			next = cleanLine(next)
			if next == line {
				break
			}
			line = next
		}

		if multi {
			if proj == "" && len(projects) == 1 {
				proj = projects[0]
			}
			if proj != "" && !hasProjectPrefix(line, proj) {
				line = proj + ": " + line
			}
		} else {
			line = stripAllProjectPrefixes(line, projects)
			line = cleanLine(line)
		}

		out = append(out, summary.WorkItem{
			Text:    line,
			Project: proj,
			Manual:  it.Manual,
			Edited:  it.Edited,
		})
	}
	return dedupeItems(out)
}

// PreferRichSummary uses fallback when the local model collapses many tasks into too few lines.
func PreferRichSummary(aiItems []summary.WorkItem, proc *activity.ProcessedActivity) []summary.WorkItem {
	aiItems = StyleItems(aiItems, proc)
	fallback := FallbackFromActivity(proc)
	want := approxTaskCount(proc)
	if len(aiItems) == 0 && len(fallback) > 0 {
		return fallback
	}
	// If model returned a single (or very few) lines but there is clearly more work, prefer fallback.
	if len(fallback) >= 3 && len(aiItems) < max(2, want/2) {
		return fallback
	}
	if len(fallback) > len(aiItems)*2 && len(aiItems) <= 2 {
		return fallback
	}
	return aiItems
}

func detectPrefix(line string) string {
	if i := strings.Index(line, ":"); i > 0 && i <= 20 {
		return strings.TrimSpace(line[:i])
	}
	return ""
}

func hasProjectPrefix(line, proj string) bool {
	return strings.HasPrefix(strings.ToLower(line), strings.ToLower(proj)+":")
}

func stripKnownPrefix(line string) string {
	if i := strings.Index(line, ":"); i > 0 && i <= 20 {
		rest := strings.TrimSpace(line[i+1:])
		if rest != "" {
			return rest
		}
	}
	return line
}

func stripAllProjectPrefixes(line string, projects []string) string {
	for _, p := range projects {
		if hasProjectPrefix(line, p) {
			return strings.TrimSpace(line[len(p)+1:])
		}
	}
	// Also strip generic "XX: " if present
	return stripKnownPrefix(line)
}

// FallbackFromActivity builds crude bullets when AI is unavailable.
func FallbackFromActivity(proc *activity.ProcessedActivity) []summary.WorkItem {
	if proc == nil {
		return nil
	}
	projects := uniqueProjects(proc)
	multi := len(projects) > 1

	var items []summary.WorkItem
	for _, c := range proc.Clusters {
		msgs := make([]string, 0, len(c.Commits))
		for _, cm := range c.Commits {
			if cm.NoiseHint {
				continue
			}
			msgs = append(msgs, cleanLine(cm.Message))
		}
		if len(msgs) == 0 && len(c.Commits) > 0 {
			msgs = append(msgs, cleanLine(c.Commits[0].Message))
		}
		if len(msgs) == 0 {
			continue
		}
		desc := msgs[0]
		if multi {
			desc = c.ShortName + ": " + desc
		}
		items = append(items, summary.WorkItem{Text: desc, Project: c.ShortName})
	}

	for _, pr := range proc.PRs {
		short := activity.ShortName(pr.RepoName)
		title := cleanLine(pr.Title)
		if title == "" {
			continue
		}
		if multi {
			title = short + ": " + title
		}
		items = append(items, summary.WorkItem{Text: title, Project: short})
	}
	return StyleItems(items, proc)
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
