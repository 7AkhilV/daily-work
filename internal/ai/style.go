package ai

import (
	"strings"

	"github.com/7AkhilV/daily-work/internal/activity"
	"github.com/7AkhilV/daily-work/internal/summary"
)

func cleanLine(s string) string {
	s = strings.TrimSpace(s)
	s = bulletRe.ReplaceAllString(s, "")
	s = leadVerbRe.ReplaceAllString(s, "")
	s = issueNumRe.ReplaceAllString(s, "")
	for bracketTagRe.MatchString(s) {
		s = strings.TrimSpace(bracketTagRe.ReplaceAllString(s, ""))
	}
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, ":")
	return strings.TrimSpace(s)
}

func humanizeLine(s string) string {
	s = cleanLine(s)
	proj := ""
	body := s
	if i := strings.Index(s, ":"); i > 0 && i <= 20 {
		cand := strings.TrimSpace(s[:i])
		if len(cand) <= 16 && !strings.Contains(cand, " ") {
			proj = cand
			body = strings.TrimSpace(s[i+1:])
		}
	}
	body = cleanLine(body)
	body = imperativeRe.ReplaceAllStringFunc(body, func(verb string) string {
		switch strings.ToLower(verb) {
		case "add":
			return "Added"
		case "enhance":
			return "Enhanced"
		case "require":
			return "Required"
		case "refactor":
			return "Refactored"
		case "update":
			return "Updated"
		case "fix":
			return "Fixed"
		case "implement":
			return "Implemented"
		case "create":
			return "Created"
		case "remove":
			return "Removed"
		case "improve":
			return "Improved"
		default:
			return verb
		}
	})
	if proj != "" {
		if strings.EqualFold(proj, "CM") {
			proj = "WC"
		}
		return proj + ": " + body
	}
	return body
}

// StyleItems applies project-prefix rules to match the user's reporting style.
func StyleItems(items []summary.WorkItem, proc *activity.ProcessedActivity) []summary.WorkItem {
	projects := uniqueProjects(proc)
	var out []summary.WorkItem
	for _, it := range items {
		line := humanizeLine(it.Text)
		if line == "" {
			continue
		}
		proj := strings.TrimSpace(it.Project)
		proj = strings.Trim(bracketTagRe.ReplaceAllString(proj, ""), "[] ")
		if proj == "" {
			proj = detectPrefix(line)
		}
		if strings.EqualFold(proj, "CM") {
			proj = "WC"
		}
		for i := 0; i < 3; i++ {
			next := cleanLine(line)
			next = stripKnownPrefix(next)
			next = cleanLine(next)
			if next == line {
				break
			}
			line = next
		}
		line = humanizeLine(line)

		if proj == "" && len(projects) >= 1 {
			proj = projects[0]
		}
		if proj == "CM" {
			proj = "WC"
		}
		if proj != "" && !hasProjectPrefix(line, proj) {
			line = proj + ": " + line
		}
		if isChoreText(line) {
			continue
		}

		out = append(out, summary.WorkItem{
			Text:    line,
			Project: proj,
			Manual:  it.Manual,
			Edited:  it.Edited,
		})
	}
	return MergeRelatedItems(dedupeItems(out))
}

// PreferRichSummary picks AI when usable; otherwise topic-based fallback.
func PreferRichSummary(aiItems []summary.WorkItem, proc *activity.ProcessedActivity) []summary.WorkItem {
	aiItems = StyleItems(aiItems, proc)
	topics := BuildTopics(proc)
	fallback := FallbackFromTopics(topics, uniqueProjects(proc))

	if len(aiItems) == 0 {
		return fallback
	}
	aiItems = ensureFeatureTopics(aiItems, fallback, topics)
	if len(aiItems) > len(topics)+2 && len(topics) >= 2 {
		return fallback
	}
	if len(topics) >= 3 && len(aiItems) < len(topics)-1 {
		return fallback
	}
	return aiItems
}

func ensureFeatureTopics(aiItems, fallback []summary.WorkItem, topics []Topic) []summary.WorkItem {
	out := aiItems
	for _, t := range topics {
		if t.Key == "docs_examples" {
			continue
		}
		if topicHasLine(out, t) {
			continue
		}
		added := false
		for _, fb := range fallback {
			if fb.Project == t.Project && topicKeyFromText(strings.ToLower(stripKnownPrefix(fb.Text))) == t.Key {
				out = append(out, fb)
				added = true
				break
			}
		}
		if added {
			continue
		}
		desc := summarizeTopic(t)
		if desc == "" || isChoreText(desc) {
			continue
		}
		line := desc
		if t.Project != "" {
			line = t.Project + ": " + desc
		}
		out = append(out, summary.WorkItem{Text: humanizeLine(line), Project: t.Project})
	}
	return out
}

func topicHasLine(items []summary.WorkItem, t Topic) bool {
	for _, it := range items {
		line := strings.ToLower(it.Text + " " + it.Project)
		switch t.Key {
		case "signed_upload":
			if strings.Contains(line, "upload") || strings.Contains(line, "signed") || strings.Contains(line, "media") {
				return true
			}
		case "job_post":
			if strings.Contains(line, "job") {
				return true
			}
		default:
			if t.Key != "" && strings.Contains(line, strings.ToLower(strings.ReplaceAll(t.Key, "_", " "))) {
				return true
			}
		}
	}
	return false
}

func isChoreText(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "nodemon") ||
		strings.Contains(lower, "development dependency") ||
		strings.Contains(lower, "unused environment") ||
		strings.Contains(lower, "unused import") ||
		strings.Contains(lower, "build script") ||
		strings.Contains(lower, "clean the build")
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
	return stripKnownPrefix(line)
}
