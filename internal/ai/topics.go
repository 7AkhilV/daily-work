package ai

import (
	"strings"

	"github.com/7AkhilV/daily-work/internal/activity"
)

// Topic is a feature-level group of commits (PRs only fill gaps).
type Topic struct {
	Key     string
	Label   string
	Project string
	Details []string // commit messages first; PR titles only if no commits
}

// BuildTopics groups COMMITS into feature topics. PR titles are used only when
// a topic/repo has no commit messages yet (secondary context).
func BuildTopics(proc *activity.ProcessedActivity) []Topic {
	if proc == nil {
		return nil
	}

	type acc struct {
		topic      Topic
		hasCommit  bool
	}
	order := []string{}
	byKey := map[string]*acc{}

	add := func(project, raw string, fromCommit bool) {
		raw = strings.TrimSpace(issueNumRe.ReplaceAllString(raw, ""))
		if raw == "" {
			return
		}
		lower := strings.ToLower(raw)
		if strings.HasPrefix(lower, "merge ") {
			return
		}
		key := project + "|" + topicKeyFromText(lower)
		a, ok := byKey[key]
		if !ok {
			a = &acc{topic: Topic{
				Key:     topicKeyFromText(lower),
				Label:   topicLabel(topicKeyFromText(lower)),
				Project: project,
			}}
			byKey[key] = a
			order = append(order, key)
		}
		if !fromCommit && a.hasCommit {
			// Already have real commits for this topic — ignore PR title noise
			return
		}
		for _, d := range a.topic.Details {
			if strings.EqualFold(d, raw) {
				return
			}
		}
		a.topic.Details = append(a.topic.Details, raw)
		if fromCommit {
			a.hasCommit = true
		}
	}

	// Commits are the source of truth
	for _, c := range proc.Clusters {
		for _, cm := range c.Commits {
			if cm.NoiseHint || cm.IsMerge {
				continue
			}
			add(c.ShortName, cm.Message, true)
		}
	}

	// PRs only if we still have no commit-backed topics covering that work
	for _, pr := range proc.PRs {
		proj := activity.ShortName(pr.RepoName)
		// Skip PR title if we already have any commits in this project today
		hasProjectCommits := false
		for _, c := range proc.Clusters {
			if c.ShortName == proj {
				for _, cm := range c.Commits {
					if !cm.NoiseHint && !cm.IsMerge {
						hasProjectCommits = true
						break
					}
				}
			}
			if hasProjectCommits {
				break
			}
		}
		if hasProjectCommits {
			continue
		}
		add(proj, pr.Title, false)
	}

	var out []Topic
	for _, k := range order {
		out = append(out, byKey[k].topic)
	}
	return out
}

func topicLabel(key string) string {
	switch key {
	case "schedule_inspect":
		return "Scheduled inspect"
	case "chat", "redis_chat":
		return "Chat module"
	case "metrics":
		return "Admin metrics / volunteers"
	case "impacts":
		return "Impacts"
	case "auth":
		return "Authentication"
	default:
		return key
	}
}
