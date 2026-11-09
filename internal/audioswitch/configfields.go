package audioswitch

import (
	"regexp"
	"strings"
)

// TopLevelScalar represents a YAML key-value pair at the top level with its
// surrounding context (indentation, inline comment).
type TopLevelScalar struct {
	Key       string
	Value     string
	Comment   string
	Line      int    // 0-based line index in the original file
	RawLine   string // the original line exactly as it appeared
	Indent    string
	IsComment bool
}

// SetTopLevelScalar updates or adds a top-level scalar key in a YAML document
// while preserving formatting, comments, and other content.
// It normalizes line endings to LF, updates existing keys in place (keeping
// their indentation and inline comment), or appends new keys at the end before
// any trailing comments.
func SetTopLevelScalar(input, key, value string) string {
	// Normalize line endings to LF
	input = strings.ReplaceAll(input, "\r\n", "\n")
	lines := strings.Split(input, "\n")

	// Regex to match a top-level key: ^(\s*)([a-zA-Z_][a-zA-Z0-9_]*):\s*(.*?)(?:\s+#(.*))?$
	keyPattern := regexp.MustCompile(`^(\s*)([a-zA-Z_][a-zA-Z0-9_]*):\s*(.*?)(?:\s+#(.*))?$`)

	found := false
	for i, line := range lines {
		matches := keyPattern.FindStringSubmatch(line)
		if matches != nil {
			existingKey := matches[2]
			if existingKey == key {
				indent := matches[1]
				comment := strings.TrimSpace(matches[4])
				newLine := indent + key + ": " + value
				if comment != "" {
					newLine += " # " + comment
				}
				lines[i] = newLine
				found = true
				break
			}
		}
	}

	if !found {
		// Append at the end, before any trailing comment-only lines
		insertIdx := len(lines)
		for i := len(lines) - 1; i >= 0; i-- {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				insertIdx = i
			} else {
				break
			}
		}
		newLine := key + ": " + value
		lines = append(lines[:insertIdx], append([]string{newLine}, lines[insertIdx:]...)...)
	}

	return strings.Join(lines, "\n")
}

// GetTopLevelScalar extracts the value of a top-level scalar key from a YAML document.
// Returns the value and true if found. Strips inline comments.
func GetTopLevelScalar(input, key string) (string, bool) {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	lines := strings.Split(input, "\n")
	keyPattern := regexp.MustCompile(`^(\s*)([a-zA-Z_][a-zA-Z0-9_]*):\s*(.*?)(?:\s+#(.*))?$`)

	for _, line := range lines {
		matches := keyPattern.FindStringSubmatch(line)
		if matches != nil {
			existingKey := matches[2]
			if existingKey == key {
				val := strings.TrimSpace(matches[3])
				// Remove trailing comment if any (already captured in group 4)
				return val, true
			}
		}
	}
	return "", false
}
