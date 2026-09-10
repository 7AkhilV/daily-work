package github

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// FetchDayActivity discovers commits (primary) and PRs (secondary context) for the day.
func (c *Client) FetchDayActivity(ctx context.Context, day time.Time, opts FetchOptions) (*DayActivity, error) {
	loc := day.Location()
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	end := start.Add(24 * time.Hour)

	login := c.Login()
	emails, err := c.userEmails(ctx)
	if err != nil {
		emails = nil
	}

	seen := map[string]struct{}{}
	var commits []CommitActivity
	addCommit := func(cm CommitActivity) {
		if cm.SHA == "" {
			return
		}
		if _, ok := seen[cm.SHA]; ok {
			return
		}
		if !opts.IncludePersonalRepos && !c.allowRepo(ctx, cm.Owner, cm.RepoName) {
			return
		}
		seen[cm.SHA] = struct{}{}
		commits = append(commits, cm)
	}

	found, err := c.searchCommits(ctx, fmt.Sprintf("author:%s", login), start, end)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch GitHub activity.\n\nPlease check your internet connection.\n\nDetails: %w", err)
	}
	for _, cm := range found {
		addCommit(cm)
	}

	found, err = c.searchCommits(ctx, fmt.Sprintf("committer:%s", login), start, end)
	if err == nil {
		for _, cm := range found {
			addCommit(cm)
		}
	}

	for _, email := range emails {
		if email == "" || strings.Contains(email, "noreply") {
			continue
		}
		found, err = c.searchCommits(ctx, fmt.Sprintf("author-email:%s", email), start, end)
		if err != nil {
			continue
		}
		for _, cm := range found {
			addCommit(cm)
		}
	}

	for _, cm := range c.commitsFromEvents(ctx, login, start, end) {
		addCommit(cm)
	}

	prs, err := c.searchPRs(ctx, login, start, end)
	if err != nil {
		prs = nil
	}
	var filteredPRs []PullRequestActivity
	for _, pr := range prs {
		owner, _ := splitFullName(pr.RepoFull)
		if !opts.IncludePersonalRepos && !c.allowRepo(ctx, owner, pr.RepoName) {
			continue
		}
		filteredPRs = append(filteredPRs, pr)
		for _, cm := range c.commitsFromPR(ctx, owner, pr.RepoName, pr.Number, login, emails, start, end) {
			addCommit(cm)
		}
	}
	prs = filteredPRs

	for i := range commits {
		if i >= 50 {
			break
		}
		if commits[i].IsMerge {
			continue
		}
		files, err := c.commitFiles(ctx, commits[i].Owner, commits[i].RepoName, commits[i].SHA)
		if err == nil {
			commits[i].Files = files
		}
	}

	repos := map[string]struct{}{}
	for _, cm := range commits {
		repos[cm.RepoFull] = struct{}{}
	}
	for _, pr := range prs {
		repos[pr.RepoFull] = struct{}{}
	}

	return &DayActivity{
		User:      login,
		Date:      start,
		Commits:   commits,
		PRs:       prs,
		RepoCount: len(repos),
	}, nil
}
