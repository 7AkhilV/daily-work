package activity

import (
	"testing"

	gh "github.com/7AkhilV/daily-work/internal/github"
)

func TestFeatureHintKeepsSignedUploadDocs(t *testing.T) {
	c := gh.CommitActivity{
		Message: "convert media uploads to signed urls",
		Files:   []gh.FileChange{{Filename: "docs/swagger.yaml"}},
	}
	if isNoise(c) {
		t.Fatal("signed upload work must not be treated as docs noise")
	}
}

func TestChoreIsNoise(t *testing.T) {
	c := gh.CommitActivity{Message: "Added nodemon as a development dependency"}
	if !isNoise(c) {
		t.Fatal("nodemon should be noise")
	}
}

func TestIsDocPath(t *testing.T) {
	if !IsDocPath("docs/swagger.yaml") || !IsDocPath("openapi.json") || IsDocPath("internal/job/edit.go") {
		t.Fatal("IsDocPath mismatch")
	}
}

func TestShortName(t *testing.T) {
	cases := map[string]string{
		"whistlingcitizen-BE":  "WC",
		"whistlingcitizen-FE":  "WC",
		"Suzhi":                "Suzhi",
		"QuestAndGames":        "QuestAndGames",
		"my-cool-backend":      "MC",
		"simple":               "Simple",
		"whistling-citizen-BE": "WC",
	}
	for in, want := range cases {
		got := ShortName(in)
		if got != want {
			t.Errorf("ShortName(%q)=%q want %q", in, got, want)
		}
	}
}
