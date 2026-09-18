package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/7AkhilV/daily-work/internal/activity"
	gh "github.com/7AkhilV/daily-work/internal/github"
	"github.com/7AkhilV/daily-work/internal/summary"
)

func TestMergeRelatedScheduleInspect(t *testing.T) {
	items := []summary.WorkItem{
		{Text: "WC: schedule inspect creation title added", Project: "WC"},
		{Text: "WC: schedule inspect creation title updated", Project: "WC"},
		{Text: "WC: Add scheduled inspect live flow and admin panel integration", Project: "WC"},
		{Text: "WC: Chat module fix", Project: "WC"},
	}
	out := MergeRelatedItems(items)
	if len(out) != 2 {
		t.Fatalf("got %d items want 2: %+v", len(out), out)
	}
}

func TestBuildTopicsPrefersCommitsOverPRTitles(t *testing.T) {
	proc := &activity.ProcessedActivity{
		Clusters: []activity.Cluster{{
			ShortName: "WC",
			Commits: []activity.ProcessedCommit{
				{CommitActivity: gh.CommitActivity{Message: "fix chat persistence for disconnect"}},
				{CommitActivity: gh.CommitActivity{Message: "add scheduled inspect live status endpoint"}},
			},
		}},
		PRs: []gh.PullRequestActivity{
			{RepoName: "whistlingcitizen-BE", Title: "Add scheduled inspect live flow and admin panel integration"},
			{RepoName: "whistlingcitizen-BE", Title: "Chat module fix"},
		},
	}
	topics := BuildTopics(proc)
	for _, tp := range topics {
		for _, d := range tp.Details {
			// PR titles should not appear when commits exist for WC
			if strings.Contains(d, "admin panel integration") || d == "Chat module fix" {
				t.Fatalf("PR title leaked into topics: %q (topics=%+v)", d, topics)
			}
		}
	}
	if len(topics) < 2 {
		t.Fatalf("expected commit-based topics, got %+v", topics)
	}
}

func TestBuildTopicsIgnoresYesterdayPR(t *testing.T) {
	loc := time.FixedZone("IST", 5*3600+30*60)
	today := time.Date(2026, 9, 18, 0, 0, 0, 0, loc)
	proc := &activity.ProcessedActivity{
		Raw: &gh.DayActivity{Date: today},
		PRs: []gh.PullRequestActivity{
			{
				RepoName:  "whistlingcitizen-BE",
				Title:     "Registration form feature added",
				CreatedAt: time.Date(2026, 9, 17, 16, 0, 0, 0, loc),
				UpdatedAt: time.Date(2026, 9, 17, 20, 0, 0, 0, loc),
			},
		},
	}
	topics := BuildTopics(proc)
	if len(topics) != 0 {
		t.Fatalf("yesterday PR should not become today's work, got %+v", topics)
	}
}

func TestBuildTopicsPrefersFeaturesOverSwaggerExamples(t *testing.T) {
	proc := &activity.ProcessedActivity{
		Clusters: []activity.Cluster{{
			ShortName: "MAB",
			Commits: []activity.ProcessedCommit{
				{CommitActivity: gh.CommitActivity{
					Message: "convert media upload to signed url uploads",
					Files:   []gh.FileChange{{Filename: "internal/upload/signed.go"}},
				}},
				{CommitActivity: gh.CommitActivity{
					Message: "job post editing APIs",
					Files:   []gh.FileChange{{Filename: "internal/job/edit.go"}},
				}},
				{CommitActivity: gh.CommitActivity{
					Message: "Enhanced API documentation with example data for requests and responses",
					Files:   []gh.FileChange{{Filename: "docs/swagger.yaml"}},
					// swagger-only is noise
				}},
			},
		}},
	}
	// mark swagger commit as noise the same way Process() would
	proc.Clusters[0].Commits[2].NoiseHint = true
	topics := BuildTopics(proc)
	got := map[string]bool{}
	for _, tp := range topics {
		got[tp.Key] = true
		for _, d := range tp.Details {
			if strings.Contains(strings.ToLower(d), "example data") {
				t.Fatalf("swagger example leaked: %+v", topics)
			}
		}
	}
	if !got["signed_upload"] || !got["job_post"] {
		t.Fatalf("expected signed_upload and job_post, got %+v", topics)
	}
	if got["docs_examples"] {
		t.Fatalf("docs_examples should be dropped when features exist: %+v", topics)
	}
}

func TestStyleItemsAlwaysPrefixes(t *testing.T) {
	proc := &activity.ProcessedActivity{
		Clusters: []activity.Cluster{{
			ShortName: "MAB",
			Commits: []activity.ProcessedCommit{
				{CommitActivity: gh.CommitActivity{Message: "job post editing APIs"}},
			},
		}},
	}
	out := StyleItems([]summary.WorkItem{{Text: "Job post editing APIs added", Project: "MAB"}}, proc)
	if len(out) != 1 || !strings.HasPrefix(out[0].Text, "MAB:") {
		t.Fatalf("expected MAB prefix, got %+v", out)
	}
}

func TestPreferRichSummaryInjectsSignedUpload(t *testing.T) {
	proc := &activity.ProcessedActivity{
		Clusters: []activity.Cluster{{
			ShortName: "MAB",
			Commits: []activity.ProcessedCommit{
				{CommitActivity: gh.CommitActivity{
					Message: "convert media upload to signed url uploads",
					Files:   []gh.FileChange{{Filename: "internal/upload/signed.go"}},
				}},
				{CommitActivity: gh.CommitActivity{
					Message: "job post editing APIs",
					Files:   []gh.FileChange{{Filename: "internal/job/edit.go"}},
				}},
			},
		}},
	}
	aiItems := []summary.WorkItem{{Text: "Added job editing API endpoint", Project: "MAB"}}
	out := PreferRichSummary(aiItems, proc)
	blob := ""
	for _, it := range out {
		blob += strings.ToLower(it.Text) + "\n"
		if !strings.HasPrefix(it.Text, "MAB:") {
			t.Fatalf("missing project prefix: %q", it.Text)
		}
	}
	if !strings.Contains(blob, "upload") && !strings.Contains(blob, "signed") {
		t.Fatalf("signed media uploads missing: %s", blob)
	}
}

func TestHumanizeAndNoBrackets(t *testing.T) {
	got := humanizeLine("WC: [WC] Add scheduled inspect live flow")
	if strings.Contains(got, "[WC]") {
		t.Fatalf("brackets remain: %q", got)
	}
	if !strings.Contains(got, "Added") {
		t.Fatalf("expected humanized verb: %q", got)
	}
}
