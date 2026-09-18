package github

import "time"

func dayBounds(day time.Time) (start, end time.Time) {
	loc := day.Location()
	start = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	end = start.Add(24 * time.Hour)
	return start, end
}

// searchRange is GitHub's inclusive timestamp range for local [start, end).
// Date-only UTC ranges are wrong for IST: "today" includes most of yesterday UTC.
func searchRange(start, end time.Time) (from, to string) {
	from = start.UTC().Format("2006-01-02T15:04:05Z")
	to = end.Add(-time.Second).UTC().Format("2006-01-02T15:04:05Z")
	return from, to
}

func inWindow(ts, start, end time.Time) bool {
	if ts.IsZero() {
		return false
	}
	return !ts.Before(start) && ts.Before(end)
}
