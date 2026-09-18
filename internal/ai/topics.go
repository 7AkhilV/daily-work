package ai

import (
	"strings"
	"time"

	"github.com/7AkhilV/daily-work/internal/activity"
	gh "github.com/7AkhilV/daily-work/internal/github"
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
		topic     Topic
		hasCommit bool
	}
	order := []string{}
	byKey := map[string]*acc{}

	add := func(project, raw string, files []gh.FileChange, fromCommit bool) {
		raw = strings.TrimSpace(issueNumRe.ReplaceAllString(raw, ""))
		if raw == "" {
			return
		}
		lower := strings.ToLower(raw)
		if strings.HasPrefix(lower, "merge ") {
			return
		}
		keyName := topicKeyFromCommit(raw, files)
		key := project + "|" + keyName
		a, ok := byKey[key]
		if !ok {
			a = &acc{topic: Topic{
				Key:     keyName,
				Label:   topicLabel(keyName),
				Project: project,
			}}
			byKey[key] = a
			order = append(order, key)
		}
		if !fromCommit && a.hasCommit {
			return
		}
		for _, d := range a.topic.Details {
			if strings.EqualFold(d, raw) {
				return
			}
		}
		a.topic.Details = append(a.topic.Details, raw)
		if paths := codeFileSummary(files); paths != "" {
			dup := false
			for _, d := range a.topic.Details {
				if d == paths {
					dup = true
					break
				}
			}
			if !dup {
				a.topic.Details = append(a.topic.Details, paths)
			}
		}
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
			add(c.ShortName, cm.Message, cm.Files, true)
		}
	}

	// PR titles only for PRs opened today with no commits yet.
	for _, pr := range proc.PRs {
		if proc.Raw != nil && !sameLocalDay(pr.CreatedAt, proc.Raw.Date) {
			continue
		}
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
		add(proj, pr.Title, nil, false)
	}

	hasFeature := map[string]bool{}
	for _, k := range order {
		t := byKey[k].topic
		if t.Key != "docs_examples" {
			hasFeature[t.Project] = true
		}
	}
	var out []Topic
	for _, k := range order {
		t := byKey[k].topic
		if t.Key == "docs_examples" && hasFeature[t.Project] {
			continue
		}
		out = append(out, t)
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
	case "signed_upload":
		return "Signed media uploads"
	case "job_post":
		return "Job post editing APIs"
	default:
		return key
	}
}

func topicKeyFromCommit(msg string, files []gh.FileChange) string {
	var b strings.Builder
	b.WriteString(msg)
	for _, f := range files {
		b.WriteByte(' ')
		b.WriteString(f.Filename)
	}
	return topicKeyFromText(b.String())
}

func codeFileSummary(files []gh.FileChange) string {
	var names []string
	for _, f := range files {
		if activity.IsDocPath(f.Filename) {
			continue
		}
		names = append(names, f.Filename)
		if len(names) == 8 {
			break
		}
	}
	if len(names) == 0 {
		return ""
	}
	return "changed files: " + strings.Join(names, ", ")
}

func sameLocalDay(ts, day time.Time) bool {
	if ts.IsZero() || day.IsZero() {
		return false
	}
	loc := day.Location()
	a := ts.In(loc)
	b := day.In(loc)
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}
