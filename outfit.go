package main

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

// An Outfit is a declarative description of a single opencode provider plus an
// optional model family and/or model — the file equivalent of one `oc-config
// add` invocation. It uses a flat, Dockerfile-style syntax:
//
//	# point opencode at one provider
//	PROVIDER openrouter
//	FAMILY   deepseek-v4
//	MODEL    deepseek/deepseek-v4-pro   # optional; sets the default
//
// Keywords are matched case-insensitively, but UPPERCASE is canonical (it is
// what `oc-config export` emits). Blank lines, full-line `#` comments, and
// trailing ` #` comments are ignored.

// Outfit keywords, in their canonical (lower-cased) form for matching.
const (
	kwProvider = "provider"
	kwFamily   = "family"
	kwModel    = "model"
)

// DefaultOutfitFile is the filename `oc-config apply` looks for when no path is
// given.
const DefaultOutfitFile = "Outfit"

// parseOutfit parses an Outfit file into a selection. It enforces that the file
// names exactly one provider and sets each instruction at most once.
func parseOutfit(data []byte) (selection, error) {
	var sel selection
	seen := map[string]int{} // keyword -> line it first appeared on

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(stripComment(scanner.Text()))
		if text == "" {
			continue
		}

		fields := strings.Fields(text)
		kw := strings.ToLower(fields[0])
		switch {
		case len(fields) < 2:
			return selection{}, fmt.Errorf("line %d: %s needs a value", line, strings.ToUpper(kw))
		case len(fields) > 2:
			return selection{}, fmt.Errorf("line %d: %s takes a single value, got %d", line, strings.ToUpper(kw), len(fields)-1)
		}
		value := fields[1]

		if prev, ok := seen[kw]; ok {
			return selection{}, fmt.Errorf("line %d: duplicate %s (already set on line %d)", line, strings.ToUpper(kw), prev)
		}

		switch kw {
		case kwProvider:
			sel.provider = value
		case kwFamily:
			sel.family = value
		case kwModel:
			sel.model = value
		default:
			return selection{}, fmt.Errorf("line %d: unknown keyword %q (expected PROVIDER, FAMILY, or MODEL)", line, fields[0])
		}
		seen[kw] = line
	}
	if err := scanner.Err(); err != nil {
		return selection{}, err
	}

	if sel.provider == "" {
		return selection{}, fmt.Errorf("Outfit is missing a PROVIDER instruction")
	}
	return sel, nil
}

// stripComment removes a comment from an Outfit line. A line whose first
// non-blank character is `#` is dropped entirely; otherwise a trailing ` #`
// (or tab-`#`) comment is removed. Provider, family, and model identifiers
// never contain spaces, so this cannot truncate a real value.
func stripComment(s string) string {
	if t := strings.TrimLeft(s, " \t"); strings.HasPrefix(t, "#") {
		return ""
	}
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		if j := strings.Index(s[i:], "#"); j >= 0 {
			return s[:i+j]
		}
	}
	return s
}

// formatOutfit renders a selection as a canonical, UPPERCASE Outfit file.
func formatOutfit(sel selection) string {
	var b strings.Builder
	fmt.Fprintf(&b, "PROVIDER %s\n", sel.provider)
	if sel.family != "" {
		fmt.Fprintf(&b, "FAMILY   %s\n", sel.family)
	}
	if sel.model != "" {
		fmt.Fprintf(&b, "MODEL    %s\n", sel.model)
	}
	return b.String()
}
