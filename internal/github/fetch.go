package github

import (
	"context"
	"fmt"
	"time"
)

// FetchDayActivity discovers commits (primary) and PRs (secondary context) for the day.
func (c *Client) FetchDayActivity(ctx context.Context, day time.Time, opts FetchOptions) (*DayActivity, error) {
	start, end := dayBounds(day)

	login := c.Login()
	emails, err := c.userEmails(ctx)
	if err != nil {
		emails = nil
	}
	emails = appendNoreplyEmails(emails, login, c.user.GetID())

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
		if email == "" {
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

	for _, org := range c.orgLogins(ctx) {
		found, err = c.searchCommits(ctx, fmt.Sprintf("author:%s org:%s", login, org), start, end)
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

	candidates, orgs := c.candidateRepos(ctx, start)
	var checked []string
	prSeen := map[string]struct{}{}
	var filteredPRs []PullRequestActivity
	addPR := func(pr PullRequestActivity) {
		key := pr.RepoFull + "#" + fmt.Sprintf("%d", pr.Number)
		if _, ok := prSeen[key]; ok {
			return
		}
		prSeen[key] = struct{}{}
		filteredPRs = append(filteredPRs, pr)
	}

	for _, pair := range candidates {
		owner, name := pair[0], pair[1]
		if !opts.IncludePersonalRepos && !c.allowRepo(ctx, owner, name) {
			continue
		}
		checked = append(checked, owner+"/"+name)
		before := len(seen)
		for _, cm := range c.commitsFromRepoEvents(ctx, owner, name, login, start, end) {
			addCommit(cm)
		}
		for _, cm := range c.commitsFromRepo(ctx, owner, name, login, emails, start, end, "") {
			addCommit(cm)
		}
		for _, pr := range c.recentPRs(ctx, owner, name, start, end) {
			hadCommit := false
			for _, cm := range c.commitsFromPR(ctx, owner, name, pr.Number, login, emails, start, end) {
				addCommit(cm)
				hadCommit = true
			}
			if hadCommit || inWindow(pr.CreatedAt, start, end) {
				addPR(pr)
			}
		}
		// Feature-branch work is invisible on the default branch; scan refs if needed.
		if len(seen) == before {
			for _, cm := range c.commitsFromRepoBranches(ctx, owner, name, login, emails, start, end) {
				addCommit(cm)
			}
		}
	}

	prs, err := c.searchPRs(ctx, login, start, end)
	if err != nil {
		prs = nil
	}
	for _, pr := range prs {
		owner, _ := splitFullName(pr.RepoFull)
		if !opts.IncludePersonalRepos && !c.allowRepo(ctx, owner, pr.RepoName) {
			continue
		}
		hadCommit := false
		for _, cm := range c.commitsFromPR(ctx, owner, pr.RepoName, pr.Number, login, emails, start, end) {
			addCommit(cm)
			hadCommit = true
		}
		if hadCommit || inWindow(pr.CreatedAt, start, end) {
			addPR(pr)
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
		User:         login,
		Date:         start,
		Commits:      commits,
		PRs:          prs,
		RepoCount:    len(repos),
		Orgs:         orgs,
		CheckedRepos: checked,
	}, nil
}
