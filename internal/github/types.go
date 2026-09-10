package github

import (
	"time"
)

// FetchOptions controls which repositories are included.
type FetchOptions struct {
	// IncludePersonalRepos when false (default) skips commits/PRs on user-owned
	// repos (e.g. 7AkhilV/daily-work) and keeps organization repos only.
	IncludePersonalRepos bool
}

type CommitActivity struct {
	SHA       string
	Message   string
	RepoFull  string // owner/repo
	RepoName  string
	Owner     string
	URL       string
	Timestamp time.Time
	Files     []FileChange
	IsMerge   bool
}

type FileChange struct {
	Filename  string
	Status    string
	Additions int
	Deletions int
	Patch     string
}

type PullRequestActivity struct {
	Number    int
	Title     string
	Body      string
	RepoFull  string
	RepoName  string
	URL       string
	Merged    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type DayActivity struct {
	User      string
	Date      time.Time
	Commits   []CommitActivity
	PRs       []PullRequestActivity
	RepoCount int
}
