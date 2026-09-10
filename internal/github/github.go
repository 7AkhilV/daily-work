package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"
)

type Client struct {
	gh   *github.Client
	user *github.User
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

func NewClient(ctx context.Context, token string) (*Client, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	gh := github.NewClient(tc)

	user, _, err := gh.Users.Get(ctx, "")
	if err != nil {
		if isUnauthorized(err) {
			return nil, fmt.Errorf("GitHub authentication expired.\n\nRun:\n\n  daily-work auth")
		}
		return nil, fmt.Errorf("unable to fetch GitHub user: %w", err)
	}
	return &Client{gh: gh, user: user}, nil
}

func (c *Client) Login() string {
	return c.user.GetLogin()
}

func (c *Client) Validate(ctx context.Context) (string, error) {
	user, _, err := c.gh.Users.Get(ctx, "")
	if err != nil {
		if isUnauthorized(err) {
			return "", fmt.Errorf("GitHub authentication expired.\n\nRun:\n\n  daily-work auth")
		}
		return "", err
	}
	return user.GetLogin(), nil
}

// FetchDayActivity discovers commits and PRs authored by the user for the local calendar day.
func (c *Client) FetchDayActivity(ctx context.Context, day time.Time) (*DayActivity, error) {
	loc := day.Location()
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	end := start.Add(24 * time.Hour)

	login := c.Login()
	emails, err := c.userEmails(ctx)
	if err != nil {
		emails = nil
	}

	commits, err := c.searchCommits(ctx, login, emails, start, end)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch GitHub activity.\n\nPlease check your internet connection.\n\nDetails: %w", err)
	}

	// Enrich with files/patches (best effort, capped).
	for i := range commits {
		if i >= 40 {
			break
		}
		files, err := c.commitFiles(ctx, commits[i].Owner, commits[i].RepoName, commits[i].SHA)
		if err == nil {
			commits[i].Files = files
		}
	}

	prs, err := c.searchPRs(ctx, login, start, end)
	if err != nil {
		prs = nil // PRs are secondary; don't fail the whole fetch
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

func (c *Client) userEmails(ctx context.Context) ([]string, error) {
	emails, _, err := c.gh.Users.ListEmails(ctx, &github.ListOptions{PerPage: 50})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range emails {
		if e.GetEmail() != "" {
			out = append(out, e.GetEmail())
		}
	}
	return out, nil
}

func (c *Client) searchCommits(ctx context.Context, login string, emails []string, start, end time.Time) ([]CommitActivity, error) {
	from := start.UTC().Format("2006-01-02")
	to := end.UTC().Add(-time.Second).Format("2006-01-02")
	q := fmt.Sprintf("author:%s author-date:%s..%s", login, from, to)
	opts := &github.SearchOptions{ListOptions: github.ListOptions{PerPage: 100}, Sort: "author-date", Order: "desc"}

	seen := map[string]struct{}{}
	var results []CommitActivity

	for {
		res, resp, err := c.gh.Search.Commits(ctx, q, opts)
		if err != nil {
			return nil, err
		}
		for _, item := range res.Commits {
			sha := item.GetSHA()
			if _, ok := seen[sha]; ok {
				continue
			}
			seen[sha] = struct{}{}

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
			// Filter to local day window precisely.
			if ts.Before(start) || !ts.Before(end) {
				continue
			}
			results = append(results, CommitActivity{
				SHA:       sha,
				Message:   firstLine(msg),
				RepoFull:  full,
				RepoName:  name,
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

	// Also try committer if author search sparse — skip for simplicity; author is primary.
	_ = emails
	return results, nil
}

func (c *Client) commitFiles(ctx context.Context, owner, repo, sha string) ([]FileChange, error) {
	commit, _, err := c.gh.Repositories.GetCommit(ctx, owner, repo, sha, nil)
	if err != nil {
		return nil, err
	}
	var files []FileChange
	for _, f := range commit.Files {
		// Filenames only — skip patches to keep memory/prompt small on low-spec Macs.
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
		repoURL := issue.GetRepositoryURL() // https://api.github.com/repos/owner/repo
		full := strings.TrimPrefix(repoURL, "https://api.github.com/repos/")
		owner, name := splitFullName(full)
		body := issue.GetBody()
		if len(body) > 800 {
			body = body[:800] + "..."
		}
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
		_ = owner
	}
	return results, nil
}

func splitFullName(full string) (owner, name string) {
	parts := strings.SplitN(full, "/", 2)
	if len(parts) != 2 {
		return "", full
	}
	return parts[0], parts[1]
}

func firstLine(msg string) string {
	msg = strings.TrimSpace(msg)
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		return strings.TrimSpace(msg[:i])
	}
	return msg
}

func isMergeMessage(msg string) bool {
	lower := strings.ToLower(firstLine(msg))
	return strings.HasPrefix(lower, "merge ") || strings.HasPrefix(lower, "merge branch")
}

func isUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	if ghErr, ok := err.(*github.ErrorResponse); ok {
		return ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusUnauthorized
	}
	return strings.Contains(err.Error(), "401")
}
