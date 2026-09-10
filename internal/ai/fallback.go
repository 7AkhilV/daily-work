package ai

import (
	"strings"

	"github.com/7AkhilV/daily-work/internal/activity"
	"github.com/7AkhilV/daily-work/internal/summary"
)

// FallbackFromActivity builds topic-grouped bullets when AI is unavailable.
func FallbackFromActivity(proc *activity.ProcessedActivity) []summary.WorkItem {
	return FallbackFromTopics(BuildTopics(proc), uniqueProjects(proc))
}

func FallbackFromTopics(topics []Topic, projects []string) []summary.WorkItem {
	multi := len(projects) > 1
	var items []summary.WorkItem
	for _, t := range topics {
		desc := summarizeTopic(t)
		if desc == "" {
			continue
		}
		proj := t.Project
		if proj == "CM" {
			proj = "WC"
		}
		text := desc
		if multi && proj != "" {
			text = proj + ": " + desc
		}
		items = append(items, summary.WorkItem{Text: humanizeLine(text), Project: proj})
	}
	return MergeRelatedItems(dedupeItems(items))
}

func summarizeTopic(t Topic) string {
	if len(t.Details) == 0 {
		return ""
	}
	best := t.Details[0]
	for _, d := range t.Details[1:] {
		if len(d) > len(best) && !isTinyTitleChange(d) {
			best = d
		}
	}
	best = humanizeLine(best)
	best = stripKnownPrefix(best)
	best = cleanLine(best)

	if t.Key == "schedule_inspect" {
		if isTinyTitleChange(best) || (strings.Contains(strings.ToLower(best), "title") && len(t.Details) > 1) {
			return "Scheduled inspect flow updates including title support and related admin/live changes"
		}
		if !strings.Contains(strings.ToLower(best), "inspect") {
			return "Scheduled inspect: " + best
		}
	}
	return best
}

func isTinyTitleChange(s string) bool {
	lower := strings.ToLower(s)
	return (strings.Contains(lower, "title added") ||
		strings.Contains(lower, "title updated") ||
		(strings.Contains(lower, "title support") && len(s) < 60)) &&
		!strings.Contains(lower, "live") &&
		!strings.Contains(lower, "admin panel")
}

// MergeRelatedItems collapses near-duplicate / same-topic lines.
func MergeRelatedItems(items []summary.WorkItem) []summary.WorkItem {
	if len(items) <= 1 {
		return items
	}
	type bucket struct {
		item summary.WorkItem
		key  string
	}
	var buckets []bucket
	for _, it := range items {
		body := strings.ToLower(stripKnownPrefix(it.Text))
		key := it.Project + "|" + topicKeyFromText(body)
		merged := false
		for i := range buckets {
			if buckets[i].key != key {
				continue
			}
			if len(it.Text) > len(buckets[i].item.Text) && !isTinyTitleChange(it.Text) {
				buckets[i].item = it
			} else if isTinyTitleChange(buckets[i].item.Text) && !isTinyTitleChange(it.Text) {
				buckets[i].item = it
			}
			merged = true
			break
		}
		if !merged {
			buckets = append(buckets, bucket{item: it, key: key})
		}
	}
	out := make([]summary.WorkItem, len(buckets))
	for i, b := range buckets {
		out[i] = b.item
	}
	return out
}

func topicKeyFromText(body string) string {
	switch {
	case strings.Contains(body, "schedule") && strings.Contains(body, "inspect"),
		strings.Contains(body, "scheduled inspect"):
		return "schedule_inspect"
	case strings.Contains(body, "chat"):
		return "chat"
	case strings.Contains(body, "metric"), strings.Contains(body, "volunteer commitment"):
		return "metrics"
	case strings.Contains(body, "impact"):
		return "impacts"
	case strings.Contains(body, "auth"):
		return "auth"
	case strings.Contains(body, "redis"):
		return "redis_chat"
	default:
		fields := strings.FieldsFunc(body, func(r rune) bool {
			return r == ' ' || r == '-' || r == ':' || r == '/'
		})
		var keep []string
		for _, f := range fields {
			if len(f) < 4 {
				continue
			}
			keep = append(keep, f)
			if len(keep) == 3 {
				break
			}
		}
		if len(keep) == 0 {
			return body
		}
		return strings.Join(keep, "_")
	}
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
