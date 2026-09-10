package activity

import (
	"regexp"
	"strings"
	"unicode"

	gh "github.com/7AkhilV/daily-work/internal/github"
)

var (
	suffixRe = regexp.MustCompile(`(?i)(-be|-fe|-backend|-frontend|-api|-app|-service|-svc|-web|-ios|-android)$`)
	noiseMsg = regexp.MustCompile(`(?i)^(merge\b|resolve conflicts|format(ting)?( code)?|prettier|eslint|chore:\s*bump|bump version|update lockfile|regenerate|generated)`)
)

type ProcessedCommit struct {
	gh.CommitActivity
	NoiseHint bool
	ShortName string
}

type Cluster struct {
	RepoFull  string
	RepoName  string
	ShortName string
	Commits   []ProcessedCommit
	NoiseHint bool
}

type ProcessedActivity struct {
	User     string
	Clusters []Cluster
	PRs      []gh.PullRequestActivity
	Raw      *gh.DayActivity
}

type ProjectNamer func(repoName string) string

func Process(day *gh.DayActivity, projectOverride ProjectNamer) *ProcessedActivity {
	if day == nil {
		return &ProcessedActivity{}
	}

	seen := map[string]struct{}{}
	var commits []ProcessedCommit
	for _, c := range day.Commits {
		if _, ok := seen[c.SHA]; ok {
			continue
		}
		seen[c.SHA] = struct{}{}
		short := resolveShortName(c.RepoName, projectOverride)
		pc := ProcessedCommit{
			CommitActivity: c,
			NoiseHint:      isNoise(c),
			ShortName:      short,
		}
		commits = append(commits, pc)
	}

	clusters := groupCommits(commits)
	return &ProcessedActivity{
		User:     day.User,
		Clusters: clusters,
		PRs:      day.PRs,
		Raw:      day,
	}
}

func resolveShortName(repoName string, override ProjectNamer) string {
	if override != nil {
		if s := override(repoName); s != "" {
			return s
		}
	}
	return ShortName(repoName)
}

// ShortName derives a project label from a repository name.
func ShortName(repoName string) string {
	name := strings.TrimSpace(repoName)
	if name == "" {
		return "Unknown"
	}
	base := suffixRe.ReplaceAllString(name, "")
	if base == "" {
		base = name
	}

	// Already short and readable (no separators, <= 12 chars)
	if !strings.ContainsAny(base, "-_.") && len(base) <= 12 {
		return capitalize(base)
	}

	parts := splitNameParts(base)
	if len(parts) == 0 {
		return truncate(base, 12)
	}
	if len(parts) == 1 {
		p := parts[0]
		if len(p) <= 12 {
			return capitalize(p)
		}
		return strings.ToUpper(string([]rune(p)[0]))
	}

	var initials strings.Builder
	for _, p := range parts {
		r := []rune(p)
		if len(r) == 0 {
			continue
		}
		initials.WriteRune(unicode.ToUpper(r[0]))
	}
	out := initials.String()
	if out == "" {
		return truncate(base, 12)
	}
	return out
}

func splitNameParts(name string) []string {
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, ".", "-")
	raw := strings.Split(name, "-")
	var parts []string
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Split camelCase / PascalCase
		parts = append(parts, splitCamel(p)...)
	}
	return parts
}

func splitCamel(s string) []string {
	var parts []string
	var cur strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && (unicode.IsLower(runes[i-1]) || (i+1 < len(runes) && unicode.IsLower(runes[i+1]))) {
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

func capitalize(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func isNoise(c gh.CommitActivity) bool {
	if c.IsMerge {
		return true
	}
	if noiseMsg.MatchString(c.Message) {
		return true
	}
	if len(c.Files) > 0 && allLockOrGenerated(c.Files) {
		return true
	}
	return false
}

func allLockOrGenerated(files []gh.FileChange) bool {
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		name := strings.ToLower(f.Filename)
		if strings.HasSuffix(name, "package-lock.json") ||
			strings.HasSuffix(name, "yarn.lock") ||
			strings.HasSuffix(name, "pnpm-lock.yaml") ||
			strings.HasSuffix(name, "go.sum") ||
			strings.HasSuffix(name, ".generated.go") ||
			strings.Contains(name, "/generated/") {
			continue
		}
		return false
	}
	return true
}

func groupCommits(commits []ProcessedCommit) []Cluster {
	if len(commits) == 0 {
		return nil
	}

	// Group by repo first, then by shared path prefix / message token overlap.
	byRepo := map[string][]ProcessedCommit{}
	order := []string{}
	for _, c := range commits {
		if _, ok := byRepo[c.RepoFull]; !ok {
			order = append(order, c.RepoFull)
		}
		byRepo[c.RepoFull] = append(byRepo[c.RepoFull], c)
	}

	var clusters []Cluster
	for _, repo := range order {
		list := byRepo[repo]
		used := make([]bool, len(list))
		for i := range list {
			if used[i] {
				continue
			}
			used[i] = true
			group := []ProcessedCommit{list[i]}
			for j := i + 1; j < len(list); j++ {
				if used[j] {
					continue
				}
				if related(list[i], list[j]) || relatedToGroup(list[j], group) {
					used[j] = true
					group = append(group, list[j])
				}
			}
			noise := true
			for _, g := range group {
				if !g.NoiseHint {
					noise = false
					break
				}
			}
			clusters = append(clusters, Cluster{
				RepoFull:  list[i].RepoFull,
				RepoName:  list[i].RepoName,
				ShortName: list[i].ShortName,
				Commits:   group,
				NoiseHint: noise,
			})
		}
	}
	return clusters
}

func related(a, b ProcessedCommit) bool {
	if pathOverlap(a.Files, b.Files) {
		return true
	}
	return messageSimilar(a.Message, b.Message)
}

func relatedToGroup(c ProcessedCommit, group []ProcessedCommit) bool {
	for _, g := range group {
		if related(c, g) {
			return true
		}
	}
	return false
}

func pathOverlap(a, b []gh.FileChange) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	dirsA := topDirs(a)
	dirsB := topDirs(b)
	for d := range dirsA {
		if dirsB[d] {
			return true
		}
	}
	// Same filenames
	names := map[string]struct{}{}
	for _, f := range a {
		names[f.Filename] = struct{}{}
	}
	for _, f := range b {
		if _, ok := names[f.Filename]; ok {
			return true
		}
	}
	return false
}

func topDirs(files []gh.FileChange) map[string]bool {
	out := map[string]bool{}
	for _, f := range files {
		parts := strings.Split(f.Filename, "/")
		if len(parts) >= 2 {
			out[parts[0]+"/"+parts[1]] = true
		} else if len(parts) == 1 {
			out[parts[0]] = true
		}
	}
	return out
}

func messageSimilar(a, b string) bool {
	ta := tokens(a)
	tb := tokens(b)
	if len(ta) == 0 || len(tb) == 0 {
		return false
	}
	overlap := 0
	for t := range ta {
		if tb[t] {
			overlap++
		}
	}
	minLen := len(ta)
	if len(tb) < minLen {
		minLen = len(tb)
	}
	return overlap >= 2 || (minLen > 0 && float64(overlap)/float64(minLen) >= 0.5)
}

func tokens(s string) map[string]bool {
	s = strings.ToLower(s)
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	stop := map[string]bool{"a": true, "an": true, "the": true, "to": true, "for": true, "and": true, "of": true, "in": true, "on": true, "fix": true, "with": true, "from": true}
	out := map[string]bool{}
	for _, f := range fields {
		if len(f) < 3 || stop[f] {
			continue
		}
		out[f] = true
	}
	return out
}
