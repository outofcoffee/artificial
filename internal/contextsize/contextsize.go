// Package contextsize parses human-friendly context window sizes and applies
// them to opencode model blocks.
package contextsize

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse parses a human-friendly context window size into a token count. It is
// deliberately lenient: surrounding whitespace, commas, and underscores are
// ignored, an optional k/m/g/t suffix is honoured (case-insensitive, decimal —
// k=1e3, m=1e6, g/b=1e9, t=1e12), a fractional value may precede a suffix
// (e.g. "1.5m"), and a trailing "tokens"/"tok" word is tolerated. So "128k",
// "128,000", "128_000", "  128 K tokens ", and "0.128m" all parse to 128000.
func Parse(s string) (int, error) {
	orig := s
	// Normalise: drop separators, lowercase, strip a trailing "tokens" word.
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer(",", "", "_", "", " ", "").Replace(s)
	for _, suf := range []string{"tokens", "token", "toks", "tok"} {
		if strings.HasSuffix(s, suf) {
			s = strings.TrimSuffix(s, suf)
			break
		}
	}

	mult := 1.0
	if n := len(s); n > 0 {
		switch s[n-1] {
		case 'k':
			mult, s = 1e3, s[:n-1]
		case 'm':
			mult, s = 1e6, s[:n-1]
		case 'g', 'b':
			mult, s = 1e9, s[:n-1]
		case 't':
			mult, s = 1e12, s[:n-1]
		}
	}

	if s == "" {
		return 0, fmt.Errorf("invalid context size %q", orig)
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid context size %q", orig)
	}
	tokens := val * mult
	if tokens < 1 {
		return 0, fmt.Errorf("context size %q must be at least 1 token", orig)
	}
	return int(tokens), nil
}

// DefaultOutput returns the output-token limit to use when the caller sets a
// context window but does not specify an output limit. opencode requires
// limit.output whenever limit.context is set; output tokens are drawn from the
// same window, so this defaults to a quarter of the context (never below one).
func DefaultOutput(ctx int) int {
	out := ctx / 4
	if out < 1 {
		out = 1
	}
	return out
}

// Apply sets limit.context and limit.output on every model in models, merging
// into any existing limit map rather than replacing it. opencode's schema
// requires output whenever context is present. The model values are the
// map[string]any entries produced by catalog.BuildProviderBlock.
func Apply(models map[string]any, ctx, output int) {
	for k, v := range models {
		m, ok := v.(map[string]any)
		if !ok {
			m = map[string]any{}
		}
		limit, ok := m["limit"].(map[string]any)
		if !ok {
			limit = map[string]any{}
		}
		limit["context"] = ctx
		limit["output"] = output
		m["limit"] = limit
		models[k] = m
	}
}
