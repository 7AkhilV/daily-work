package activity

import "testing"

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
