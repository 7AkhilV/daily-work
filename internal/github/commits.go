package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v69/github"
)

func (c *Client) searchCommits(ctx context.Context, qualifier string, start, end time.Time) ([]CommitActivity, error) {
	from := start.UTC().Format("2006-01-02")
	to := end.UTC().Add(-time.Second).Format("2006-01-02")
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
	out = append(out, c.paginatePushEvents(login, start, end, func(opts *github.ListOptions) ([]*github.Event, *github.Response, error) {
		return c.gh.Activity.ListEventsPerformedByUser(ctx, login, false, opts)
	})...)

	// Private org pushes often do not show up on /users/{user}/events.
	orgs, _, err := c.gh.Organizations.List(ctx, "", &github.ListOptions{PerPage: 100})
	if err != nil {
		return out
	}
	for _, org := range orgs {
		name := org.GetLogin()
		if name == "" {
			continue
		}
		out = append(out, c.paginatePushEvents(login, start, end, func(opts *github.ListOptions) ([]*github.Event, *github.Response, error) {
			return c.gh.Activity.ListUserEventsForOrganization(ctx, name, login, opts)
		})...)
	}
	return out
}

func (c *Client) paginatePushEvents(login string, start, end time.Time, list func(*github.ListOptions) ([]*github.Event, *github.Response, error)) []CommitActivity {
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
			if login != "" && ev.GetActor() != nil && ev.GetActor().GetLogin() != "" &&
				!strings.EqualFold(ev.GetActor().GetLogin(), login) {
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

func (c *Client) commitsFromRepo(ctx context.Context, owner, repo, login string, emails []string, start, end time.Time, ref string) []CommitActivity {
	if owner == "" || repo == "" {
		return nil
	}
	emailSet := map[string]bool{}
	for _, e := range emails {
		emailSet[strings.ToLower(e)] = true
	}
	opts := &github.CommitsListOptions{
		SHA:         ref,
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
			if login != "" && !commitByUser(item, login, emailSet) {
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

func (c *Client) commitsFromRepoBranches(ctx context.Context, owner, repo, login string, emails []string, start, end time.Time) []CommitActivity {
	var out []CommitActivity
	out = append(out, c.commitsFromRepo(ctx, owner, repo, login, emails, start, end, "")...)

	opts := &github.BranchListOptions{ListOptions: github.ListOptions{PerPage: 100}}
	branches, _, err := c.gh.Repositories.ListBranches(ctx, owner, repo, opts)
	if err != nil {
		return out
	}
	seen := map[string]struct{}{"": {}}
	for _, b := range branches {
		name := b.GetName()
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c.commitsFromRepo(ctx, owner, repo, login, emails, start, end, name)...)
	}
	return out
}

func (c *Client) commitsFromRepoEvents(ctx context.Context, owner, repo, login string, start, end time.Time) []CommitActivity {
	return c.paginatePushEvents(login, start, end, func(opts *github.ListOptions) ([]*github.Event, *github.Response, error) {
		return c.gh.Activity.ListRepositoryEvents(ctx, owner, repo, opts)
	})
}

func (c *Client) recentPRs(ctx context.Context, owner, repo string, start, end time.Time) []PullRequestActivity {
	if owner == "" || repo == "" {
		return nil
	}
	opts := &github.PullRequestListOptions{
		State:       "all",
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 30},
	}
	list, _, err := c.gh.PullRequests.List(ctx, owner, repo, opts)
	if err != nil {
		return nil
	}
	full := owner + "/" + repo
	var out []PullRequestActivity
	for _, pr := range list {
		updated := pr.GetUpdatedAt().Time
		created := pr.GetCreatedAt().Time
		if updated.Before(start) && created.Before(start) {
			break
		}
		if !inWindow(updated, start, end) && !inWindow(created, start, end) {
			continue
		}
		body := pr.GetBody()
		if len(body) > 800 {
			body = body[:800] + "..."
		}
		out = append(out, PullRequestActivity{
			Number:    pr.GetNumber(),
			Title:     pr.GetTitle(),
			Body:      body,
			RepoFull:  full,
			RepoName:  repo,
			URL:       pr.GetHTMLURL(),
			CreatedAt: created,
			UpdatedAt: updated,
		})
	}
	return out
}

func (c *Client) orgLogins(ctx context.Context) []string {
	orgs, _, err := c.gh.Organizations.List(ctx, "", &github.ListOptions{PerPage: 100})
	if err != nil {
		return nil
	}
	var out []string
	for _, org := range orgs {
		if name := org.GetLogin(); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func (c *Client) candidateRepos(ctx context.Context, start time.Time) ([][2]string, []string) {
	seen := map[string]struct{}{}
	var out [][2]string
	add := func(owner, name string) {
		owner = strings.TrimSpace(owner)
		name = strings.TrimSpace(name)
		if owner == "" || name == "" {
			return
		}
		key := strings.ToLower(owner + "/" + name)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, [2]string{owner, name})
	}
	orgs := c.orgLogins(ctx)
	for _, org := range orgs {
		for _, pair := range c.orgReposPushedSince(ctx, org, start) {
			add(pair[0], pair[1])
		}
	}
	for _, pair := range c.recentlyPushedRepos(ctx, start) {
		add(pair[0], pair[1])
	}
	return out, orgs
}

func (c *Client) orgReposPushedSince(ctx context.Context, org string, start time.Time) [][2]string {
	if org == "" {
		return nil
	}
	opts := &github.RepositoryListByOrgOptions{
		Sort:        "pushed",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 50},
	}
	var out [][2]string
	pages := 0
	for {
		repos, resp, err := c.gh.Repositories.ListByOrg(ctx, org, opts)
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
			name := r.GetName()
			owner := org
			if r.Owner != nil && r.Owner.GetLogin() != "" {
				owner = r.Owner.GetLogin()
			}
			if name == "" {
				continue
			}
			out = append(out, [2]string{owner, name})
			if len(out) >= 20 {
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
	committer := time.Time{}
	if item.Commit != nil && item.Commit.Committer != nil {
		committer = item.Commit.Committer.GetDate().Time
	}
	if !inWindow(ts, start, end) && !inWindow(committer, start, end) {
		return nil
	}
	if ts.IsZero() {
		ts = committer
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
		if login != "" && item.Commit.Author != nil && strings.EqualFold(item.Commit.Author.GetName(), login) {
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
	from := start.UTC().Format("2006-01-02")
	to := end.UTC().Add(-time.Second).Format("2006-01-02")
	// involves: catches PRs you opened or pushed to; author: alone misses collaborator work.
	// Date-only search is UTC and can include yesterday in IST — inWindow trims that.
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
