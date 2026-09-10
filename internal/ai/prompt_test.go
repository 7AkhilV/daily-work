package ai

import (
	"strings"
	"testing"

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

func TestHumanizeAndNoBrackets(t *testing.T) {
	got := humanizeLine("WC: [WC] Add scheduled inspect live flow")
	if strings.Contains(got, "[WC]") {
		t.Fatalf("brackets remain: %q", got)
	}
	if !strings.Contains(got, "Added") {
		t.Fatalf("expected humanized verb: %q", got)
	}
}
