package audioswitch

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type SliderMappingEntry struct {
	Index   int
	Targets []string
}

// ParseSliderMapping parses the slider_mapping section from a deej config.yaml string.
// Returns a slice of SliderMappingEntry sorted by Index.
func ParseSliderMapping(config string) ([]SliderMappingEntry, error) {
	config = strings.ReplaceAll(config, "\r\n", "\n")
	lines := strings.Split(config, "\n")

	inSliderMapping := false
	var entries []SliderMappingEntry
	indexRe := regexp.MustCompile(`^\s+(\d+):\s*(.*?)\s*$`)
	targetRe := regexp.MustCompile(`^\s+-\s*(.+?)\s*$`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inSliderMapping {
			if trimmed == "slider_mapping:" {
				inSliderMapping = true
			}
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Check if we've left the slider_mapping section (new top-level key)
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
		if m := indexRe.FindStringSubmatch(line); m != nil {
			idx, err := strconv.Atoi(m[1])
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid slider index %q", i+1, m[1])
			}
			entry := SliderMappingEntry{Index: idx}
			if value := scalarTarget(m[2]); value != "" {
				entry.Targets = []string{value}
			}
			entries = append(entries, entry)
		} else if m := targetRe.FindStringSubmatch(line); m != nil && len(entries) > 0 {
			if target := scalarTarget(m[1]); target != "" {
				entries[len(entries)-1].Targets = append(entries[len(entries)-1].Targets, target)
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Index < entries[j].Index })
	return entries, nil
}

// scalarTarget cleans an inline YAML scalar value: strips comments and quotes.
func scalarTarget(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.Index(value, " #"); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}
	return strings.Trim(strings.TrimSpace(value), `"'`)
}

// ReplaceSliderMapping replaces the slider_mapping section in a config.yaml string
// with the given entries, preserving all other content and formatting as much as possible.
func ReplaceSliderMapping(config string, entries []SliderMappingEntry) (string, error) {
	config = strings.ReplaceAll(config, "\r\n", "\n")
	lines := strings.Split(config, "\n")

	inSliderMapping := false
	start := -1
	end := -1
	indent := ""

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "slider_mapping:" {
			inSliderMapping = true
			start = i
			// Detect indent of the first child (next non-empty line)
			for j := i + 1; j < len(lines); j++ {
				t := strings.TrimSpace(lines[j])
				if t != "" && !strings.HasPrefix(t, "#") {
					indent = lines[j][:len(lines[j])-len(t)]
					break
				}
			}
			continue
		}
		if inSliderMapping {
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && trimmed != "" {
				end = i
				break
			}
		}
	}
	if end == -1 {
		end = len(lines)
	}
	if indent == "" {
		indent = "  "
	}

	if start == -1 {
		// No slider_mapping section found; append at end
		var b strings.Builder
		b.WriteString(config)
		if !strings.HasSuffix(config, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("slider_mapping:\n")
		for _, e := range entries {
			b.WriteString(fmt.Sprintf("%s%d:\n", indent, e.Index))
			for _, t := range e.Targets {
				b.WriteString(fmt.Sprintf("%s  - %s\n", indent, t))
			}
		}
		return b.String(), nil
	}

	// Build new slider_mapping section
	var newLines []string
	newLines = append(newLines, lines[:start+1]...) // include "slider_mapping:"
	for _, e := range entries {
		newLines = append(newLines, fmt.Sprintf("%s%d:", indent, e.Index))
		for _, t := range e.Targets {
			newLines = append(newLines, fmt.Sprintf("%s  - %s", indent, t))
		}
	}
	newLines = append(newLines, lines[end:]...)

	return strings.Join(newLines, "\n"), nil
}
