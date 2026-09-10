package activity

import "testing"

func TestShortName(t *testing.T) {
	cases := map[string]string{
		// Long single token after stripping -BE → first initial (override via config for "WC")
		"whistlingcitizen-BE": "W",
		"Suzhi":               "Suzhi",
		"QuestAndGames":       "QAG",
		"my-cool-backend":     "MC",
		"simple":              "Simple",
		"whistling-citizen-BE": "WC",
	}
	for in, want := range cases {
		got := ShortName(in)
		if got != want {
			t.Errorf("ShortName(%q)=%q want %q", in, got, want)
		}
	}
}
