package github

import (
	"fmt"
	"strings"
)

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

func appendNoreplyEmails(emails []string, login string, userID int64) []string {
	login = strings.TrimSpace(login)
	extra := []string{}
	if login != "" {
		extra = append(extra, login+"@users.noreply.github.com")
		if userID != 0 {
			extra = append(extra, fmt.Sprintf("%d+%s@users.noreply.github.com", userID, login))
		}
	}
	seen := map[string]struct{}{}
	var out []string
	for _, e := range append(emails, extra...) {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" {
			continue
		}
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	return out
}
