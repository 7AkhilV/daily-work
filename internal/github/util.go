package github

import "strings"

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
