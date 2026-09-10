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
	if !ts.IsZero() && (ts.Before(start) || !ts.Before(end)) {
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
	opts := &github.ListOptions{PerPage: 100}
	var out []CommitActivity
	pages := 0
	for {
		events, resp, err := c.gh.Activity.ListEventsPerformedByUser(ctx, login, false, opts)
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
			if !created.Before(end) {
				continue
			}
			if ev.GetType() != "PushEvent" {
				continue
			}
			raw, err := ev.ParsePayload()
			if err != nil {
				continue
			}
			push, ok := raw.(*github.PushEvent)
			if !ok || push == nil {
				continue
			}
			repo := ev.GetRepo()
			full := repo.GetName()
			if !strings.Contains(full, "/") {
				full = repo.GetFullName()
			}
			owner, name := splitFullName(full)
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
		}
		if stop || resp.NextPage == 0 || pages >= 3 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out
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
			if ts.IsZero() || ts.Before(start) || !ts.Before(end) {
				continue
			}
			sha := item.GetSHA()
			full := owner + "/" + repo
			out = append(out, CommitActivity{
				SHA:       sha,
				Message:   firstLine(msg),
				RepoFull:  full,
				RepoName:  repo,
				Owner:     owner,
				URL:       item.GetHTMLURL(),
				Timestamp: ts,
				IsMerge:   isMergeMessage(msg),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out
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
	q := fmt.Sprintf("author:%s type:pr updated:%s..%s", login, start.UTC().Format("2006-01-02"), end.UTC().Format("2006-01-02"))
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
			CreatedAt: issue.GetCreatedAt().Time,
			UpdatedAt: issue.GetUpdatedAt().Time,
		})
	}
	return results, nil
}
