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
	multi := len(projects) > 1
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

		if multi {
			if proj == "" && len(projects) == 1 {
				proj = projects[0]
			}
			if proj == "CM" {
				proj = "WC"
			}
			if proj != "" && !hasProjectPrefix(line, proj) {
				line = proj + ": " + line
			}
		} else {
			line = stripAllProjectPrefixes(line, projects)
			line = cleanLine(line)
			line = humanizeLine(line)
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
	if len(aiItems) > len(topics)+2 && len(topics) >= 2 {
		return fallback
	}
	if len(topics) >= 3 && len(aiItems) < len(topics)-1 {
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
	return stripKnownPrefix(line)
}
