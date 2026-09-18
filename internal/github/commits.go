package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v69/github"
)

func (c *Client) searchCommits(ctx context.Context, qualifier string, start, end time.Time) ([]CommitActivity, error) {
	from, to := searchRange(start, end)
	results, err := c.searchCommitsQuery(ctx, qualifier, from, to, start, end)
	if err == nil {
		return results, nil
	}
	// Some GitHub search backends only accept YYYY-MM-DD; still post-filter to local day.
	from = start.UTC().Format("2006-01-02")
	to = end.UTC().Add(-time.Second).Format("2006-01-02")
	return c.searchCommitsQuery(ctx, qualifier, from, to, start, end)
}

func (c *Client) searchCommitsQuery(ctx context.Context, qualifier, from, to string, start, end time.Time) ([]CommitActivity, error) {
	q := fmt.Sprintf("%s author-date:%s..%s", qualifier, from, to)
	if strings.HasPrefix(qualifier, "committer:") || strings.HasPrefix(qualifier, "committer-email:") {
		q = fmt.Sprintf("%s committer-date:%s..%s", qualifier, from, to)
	}
	opts := &github.SearchOptions{ListOptions: github.ListOptions{PerPage: 100}, Sort: "author-date", Order: "desc"}

	var results []CommitActivity
	for {
		res, resp, err := c.gh.Search.Commits(ctx, q, opts)
		if err != nil {
			return nil, err
		}
		for _, item := range res.Commits {
			cm := commitFromSearch(item, start, end)
			if cm != nil {
				results = append(results, *cm)
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return results, nil
}

func commitFromSearch(item *github.CommitResult, start, end time.Time) *CommitActivity {
	if item == nil {
		return nil
	}
	sha := item.GetSHA()
	repo := item.GetRepository()
	full := repo.GetFullName()
	owner, name := splitFullName(full)
	msg := ""
	if item.Commit != nil {
		msg = item.Commit.GetMessage()
	}
	ts := time.Time{}
	if item.Commit != nil && item.Commit.Author != nil {
		ts = item.Commit.Author.GetDate().Time
	}
	if ts.IsZero() && item.Commit != nil && item.Commit.Committer != nil {
		ts = item.Commit.Committer.GetDate().Time
	}
	if !ts.IsZero() && !inWindow(ts, start, end) {
		return nil
	}
	return &CommitActivity{
		SHA:       sha,
		Message:   firstLine(msg),
		RepoFull:  full,
		RepoName:  name,
		Owner:     owner,
		URL:       item.GetHTMLURL(),
		Timestamp: ts,
		IsMerge:   isMergeMessage(msg),
	}
}

func (c *Client) commitsFromEvents(ctx context.Context, login string, start, end time.Time) []CommitActivity {
	var out []CommitActivity
	out = append(out, c.paginatePushEvents(start, end, func(opts *github.ListOptions) ([]*github.Event, *github.Response, error) {
		return c.gh.Activity.ListEventsPerformedByUser(ctx, login, false, opts)
	})...)

	// Private org pushes often do not show up on /users/{user}/events.
	orgs, err := c.gh.Organizations.List(ctx, "", &github.ListOptions{PerPage: 100})
	if err != nil {
		return out
	}
	for _, org := range orgs {
		name := org.GetLogin()
		if name == "" {
			continue
		}
		out = append(out, c.paginatePushEvents(start, end, func(opts *github.ListOptions) ([]*github.Event, *github.Response, error) {
			return c.gh.Activity.ListUserEventsForOrganization(ctx, name, login, opts)
		})...)
	}
	return out
}

func (c *Client) paginatePushEvents(start, end time.Time, list func(*github.ListOptions) ([]*github.Event, *github.Response, error)) []CommitActivity {
	opts := &github.ListOptions{PerPage: 100}
	var out []CommitActivity
	pages := 0
	for {
		events, resp, err := list(opts)
		if err != nil {
			return out
		}
		pages++
		stop := false
		for _, ev := range events {
			created := ev.GetCreatedAt().Time
			if created.Before(start) {
				stop = true
				break
			}
			if !inWindow(created, start, end) {
				continue
			}
			out = append(out, commitsFromPushEvent(ev, created)...)
		}
		if stop || resp.NextPage == 0 || pages >= 5 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out
}

func commitsFromPushEvent(ev *github.Event, created time.Time) []CommitActivity {
	if ev == nil || ev.GetType() != "PushEvent" {
		return nil
	}
	raw, err := ev.ParsePayload()
	if err != nil {
		return nil
	}
	push, ok := raw.(*github.PushEvent)
	if !ok || push == nil {
		return nil
	}
	full := eventRepoFull(ev.GetRepo())
	owner, name := splitFullName(full)
	var out []CommitActivity
	for _, pc := range push.Commits {
		msg := pc.GetMessage()
		sha := pc.GetSHA()
		out = append(out, CommitActivity{
			SHA:       sha,
			Message:   firstLine(msg),
			RepoFull:  full,
			RepoName:  name,
			Owner:     owner,
			URL:       fmt.Sprintf("https://github.com/%s/commit/%s", full, sha),
			Timestamp: created,
			IsMerge:   isMergeMessage(msg),
		})
	}
	return out
}

func eventRepoFull(repo *github.Repository) string {
	if repo == nil {
		return ""
	}
	if full := repo.GetFullName(); strings.Contains(full, "/") {
		return full
	}
	if name := repo.GetName(); strings.Contains(name, "/") {
		return name
	}
	url := strings.TrimPrefix(repo.GetURL(), "https://api.github.com/repos/")
	if strings.Contains(url, "/") && !strings.Contains(url, "://") {
		return url
	}
	if repo.Owner != nil && repo.Owner.GetLogin() != "" && repo.GetName() != "" {
		return repo.Owner.GetLogin() + "/" + repo.GetName()
	}
	return repo.GetName()
}

func (c *Client) commitsFromPR(ctx context.Context, owner, repo string, number int, login string, emails []string, start, end time.Time) []CommitActivity {
	if owner == "" || repo == "" || number == 0 {
		return nil
	}
	opts := &github.ListOptions{PerPage: 100}
	var out []CommitActivity
	emailSet := map[string]bool{}
	for _, e := range emails {
		emailSet[strings.ToLower(e)] = true
	}

	for {
		list, resp, err := c.gh.PullRequests.ListCommits(ctx, owner, repo, number, opts)
		if err != nil {
			return out
		}
		for _, item := range list {
			if !commitByUser(item, login, emailSet) {
				continue
			}
			if cm := commitFromRepo(item, owner, repo, start, end); cm != nil {
				out = append(out, *cm)
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out
}

func (c *Client) commitsFromRepo(ctx context.Context, owner, repo, login string, start, end time.Time) []CommitActivity {
	if owner == "" || repo == "" || login == "" {
		return nil
	}
	opts := &github.CommitsListOptions{
		Author:      login,
		Since:       start,
		Until:       end,
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var out []CommitActivity
	for {
		list, resp, err := c.gh.Repositories.ListCommits(ctx, owner, repo, opts)
		if err != nil {
			return out
		}
		for _, item := range list {
			if cm := commitFromRepo(item, owner, repo, start, end); cm != nil {
				out = append(out, *cm)
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out
}

func (c *Client) recentlyPushedRepos(ctx context.Context, start time.Time) [][2]string {
	opts := &github.RepositoryListByAuthenticatedUserOptions{
		Sort:        "pushed",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 50},
	}
	var out [][2]string
	pages := 0
	for {
		repos, resp, err := c.gh.Repositories.ListByAuthenticatedUser(ctx, opts)
		if err != nil {
			return out
		}
		pages++
		stop := false
		for _, r := range repos {
			if !r.GetPushedAt().Time.IsZero() && r.GetPushedAt().Time.Before(start) {
				stop = true
				break
			}
			owner := ""
			if r.Owner != nil {
				owner = r.Owner.GetLogin()
			}
			name := r.GetName()
			if owner == "" || name == "" {
				owner, name = splitFullName(r.GetFullName())
			}
			if owner == "" || name == "" {
				continue
			}
			out = append(out, [2]string{owner, name})
			if len(out) >= 30 {
				stop = true
				break
			}
		}
		if stop || resp.NextPage == 0 || pages >= 2 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out
}

func commitFromRepo(item *github.RepositoryCommit, owner, repo string, start, end time.Time) *CommitActivity {
	if item == nil {
		return nil
	}
	msg := ""
	ts := time.Time{}
	if item.Commit != nil {
		msg = item.Commit.GetMessage()
		if item.Commit.Author != nil {
			ts = item.Commit.Author.GetDate().Time
		}
		if ts.IsZero() && item.Commit.Committer != nil {
			ts = item.Commit.Committer.GetDate().Time
		}
	}
	if ts.IsZero() || !inWindow(ts, start, end) {
		return nil
	}
	sha := item.GetSHA()
	full := owner + "/" + repo
	url := item.GetHTMLURL()
	if url == "" {
		url = fmt.Sprintf("https://github.com/%s/commit/%s", full, sha)
	}
	return &CommitActivity{
		SHA:       sha,
		Message:   firstLine(msg),
		RepoFull:  full,
		RepoName:  repo,
		Owner:     owner,
		URL:       url,
		Timestamp: ts,
		IsMerge:   isMergeMessage(msg),
	}
}

func commitByUser(item *github.RepositoryCommit, login string, emails map[string]bool) bool {
	if item == nil {
		return false
	}
	if item.Author != nil && strings.EqualFold(item.Author.GetLogin(), login) {
		return true
	}
	if item.Committer != nil && strings.EqualFold(item.Committer.GetLogin(), login) {
		return true
	}
	if item.Commit != nil {
		if item.Commit.Author != nil && emails[strings.ToLower(item.Commit.Author.GetEmail())] {
			return true
		}
		if item.Commit.Committer != nil && emails[strings.ToLower(item.Commit.Committer.GetEmail())] {
			return true
		}
	}
	return false
}

func (c *Client) commitFiles(ctx context.Context, owner, repo, sha string) ([]FileChange, error) {
	commit, _, err := c.gh.Repositories.GetCommit(ctx, owner, repo, sha, nil)
	if err != nil {
		return nil, err
	}
	var files []FileChange
	for _, f := range commit.Files {
		files = append(files, FileChange{
			Filename:  f.GetFilename(),
			Status:    f.GetStatus(),
			Additions: f.GetAdditions(),
			Deletions: f.GetDeletions(),
		})
		if len(files) >= 15 {
			break
		}
	}
	return files, nil
}

func (c *Client) searchPRs(ctx context.Context, login string, start, end time.Time) ([]PullRequestActivity, error) {
	from, to := searchRange(start, end)
	results, err := c.searchPRsQuery(ctx, login, from, to, start, end)
	if err == nil {
		return results, nil
	}
	from = start.UTC().Format("2006-01-02")
	to = end.UTC().Add(-time.Second).Format("2006-01-02")
	return c.searchPRsQuery(ctx, login, from, to, start, end)
}

func (c *Client) searchPRsQuery(ctx context.Context, login, from, to string, start, end time.Time) ([]PullRequestActivity, error) {
	// involves: catches PRs you opened or pushed to; author: alone misses collaborator work.
	q := fmt.Sprintf("involves:%s type:pr updated:%s..%s", login, from, to)
	opts := &github.SearchOptions{ListOptions: github.ListOptions{PerPage: 50}, Sort: "updated", Order: "desc"}

	var results []PullRequestActivity
	res, _, err := c.gh.Search.Issues(ctx, q, opts)
	if err != nil {
		return nil, err
	}
	for _, issue := range res.Issues {
		if !issue.IsPullRequest() {
			continue
		}
		created := issue.GetCreatedAt().Time
		updated := issue.GetUpdatedAt().Time
		if !inWindow(updated, start, end) && !inWindow(created, start, end) {
			continue
		}
		repoURL := issue.GetRepositoryURL()
		full := strings.TrimPrefix(repoURL, "https://api.github.com/repos/")
		body := issue.GetBody()
		if len(body) > 800 {
			body = body[:800] + "..."
		}
		_, name := splitFullName(full)
		results = append(results, PullRequestActivity{
			Number:    issue.GetNumber(),
			Title:     issue.GetTitle(),
			Body:      body,
			RepoFull:  full,
			RepoName:  name,
			URL:       issue.GetHTMLURL(),
			CreatedAt: created,
			UpdatedAt: updated,
		})
	}
	return results, nil
}
