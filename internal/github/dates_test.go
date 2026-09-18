package github

import (
	"testing"
	"time"
)

func TestSearchRangeIST(t *testing.T) {
	loc := time.FixedZone("IST", 5*3600+30*60)
	start, end := dayBounds(time.Date(2026, 9, 18, 10, 0, 0, 0, loc))
	from, to := searchRange(start, end)
	if from != "2026-09-17T18:30:00Z" {
		t.Fatalf("from=%s want 2026-09-17T18:30:00Z", from)
	}
	if to != "2026-09-18T18:29:59Z" {
		t.Fatalf("to=%s want 2026-09-18T18:29:59Z", to)
	}
}

func TestInWindowLocalDay(t *testing.T) {
	loc := time.FixedZone("IST", 5*3600+30*60)
	start, end := dayBounds(time.Date(2026, 9, 18, 0, 0, 0, 0, loc))

	yesterdayEvening := time.Date(2026, 9, 17, 20, 0, 0, 0, loc)
	todayMorning := time.Date(2026, 9, 18, 1, 0, 0, 0, loc)
	todayUTC := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC) // 15:30 IST

	if inWindow(yesterdayEvening, start, end) {
		t.Fatal("yesterday evening IST must not count as today")
	}
	if !inWindow(todayMorning, start, end) {
		t.Fatal("01:00 IST is today")
	}
	if !inWindow(todayUTC, start, end) {
		t.Fatal("10:00 UTC is still 18 Sep IST")
	}
}

func TestEventRepoFull(t *testing.T) {
	if got := eventRepoFull(nil); got != "" {
		t.Fatalf("nil repo: %q", got)
	}
}
