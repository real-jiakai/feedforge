// Package pattern implements Feed43-style search patterns.
//
// A search pattern is literal text mixed with two macros:
//
//	{%}  matches any text sequence and captures it
//	{*}  matches any text sequence and skips it
//
// Macros are lazy: they stop at the first occurrence of the following
// literal text. The single exception is a capturing "{%}" at the very end of
// a pattern, which is greedy so that a bare "{%}" grabs the entire search
// region — this mirrors the documented Feed43 behaviour of using "{%}" as the
// item pattern to turn the whole global capture into a single item. A
// trailing "{*}" stays lazy (it captures nothing, so greediness would only
// swallow the rest of the region and collapse every later match).
//
// Leading and trailing whitespace is trimmed from patterns before
// compilation: it is invisible in a textarea and would otherwise silently
// change matching (a stray newline demotes a trailing macro to an interior
// one).
package pattern

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	// MaxPatternLen bounds user-supplied pattern size.
	MaxPatternLen = 8 * 1024
	// MaxCaptures bounds the number of {%} macros per pattern.
	MaxCaptures = 50
)

// Pattern is a compiled search pattern.
type Pattern struct {
	re        *regexp.Regexp
	nCaptures int
}

// NumCaptures reports how many {%} macros the pattern contains.
func (p *Pattern) NumCaptures() int { return p.nCaptures }

var wsRun = regexp.MustCompile(`\s+`)

// Compile translates a Feed43-style pattern into a regular expression.
//
// When smartWhitespace is true, any run of whitespace in the pattern matches
// any run of whitespace in the page (so patterns survive indentation and
// CRLF/LF differences). When false, literal text must match exactly.
func Compile(pat string, smartWhitespace bool) (*Pattern, error) {
	if len(pat) > MaxPatternLen {
		return nil, fmt.Errorf("pattern too long (%d bytes, max %d)", len(pat), MaxPatternLen)
	}
	pat = strings.TrimSpace(normalizeNewlines(pat))
	if pat == "" {
		return nil, errors.New("pattern is empty")
	}

	type token struct {
		macro   string // "%" capture, "*" skip, "" literal
		literal string
	}
	var tokens []token
	rest := pat
	for rest != "" {
		iCap := strings.Index(rest, "{%}")
		iSkip := strings.Index(rest, "{*}")
		i, macro := -1, ""
		switch {
		case iCap >= 0 && (iSkip < 0 || iCap < iSkip):
			i, macro = iCap, "%"
		case iSkip >= 0:
			i, macro = iSkip, "*"
		}
		if i < 0 {
			tokens = append(tokens, token{literal: rest})
			break
		}
		if i > 0 {
			tokens = append(tokens, token{literal: rest[:i]})
		}
		tokens = append(tokens, token{macro: macro})
		rest = rest[i+3:]
	}

	var sb strings.Builder
	sb.WriteString("(?s)")
	nCaptures := 0
	for ti, tk := range tokens {
		last := ti == len(tokens)-1
		switch tk.macro {
		case "%":
			nCaptures++
			if nCaptures > MaxCaptures {
				return nil, fmt.Errorf("too many {%%} macros (max %d)", MaxCaptures)
			}
			if last {
				sb.WriteString("(.*)")
			} else {
				sb.WriteString("(.*?)")
			}
		case "*":
			// Always lazy — see the package doc. A greedy trailing skip
			// would consume the rest of the region and collapse every
			// subsequent item into nothing.
			sb.WriteString(".*?")
		default:
			sb.WriteString(escapeLiteral(tk.literal, smartWhitespace))
		}
	}

	re, err := regexp.Compile(sb.String())
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}
	return &Pattern{re: re, nCaptures: nCaptures}, nil
}

func escapeLiteral(lit string, smartWhitespace bool) string {
	if !smartWhitespace {
		return regexp.QuoteMeta(lit)
	}
	var sb strings.Builder
	rest := lit
	for rest != "" {
		loc := wsRun.FindStringIndex(rest)
		if loc == nil {
			sb.WriteString(regexp.QuoteMeta(rest))
			break
		}
		sb.WriteString(regexp.QuoteMeta(rest[:loc[0]]))
		sb.WriteString(`\s+`)
		rest = rest[loc[1]:]
	}
	return sb.String()
}

func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// NormalizePage prepares page content for matching (CRLF/CR → LF).
func NormalizePage(s string) string { return normalizeNewlines(s) }

// FindFirst applies the pattern once and returns its captures, or nil when
// the pattern does not match.
func (p *Pattern) FindFirst(page string) []string {
	m := p.re.FindStringSubmatch(page)
	if m == nil {
		return nil
	}
	return m[1:]
}

// FindAll applies the pattern repeatedly and returns the captures of each
// non-overlapping match, up to limit matches (limit <= 0 means no limit).
// Matches that consume no text are skipped.
//
// Matching stops as soon as limit matches are found, so a small limit stays
// cheap even on a page containing a huge number of potential matches.
func (p *Pattern) FindAll(page string, limit int) [][]string {
	var out [][]string
	p.each(page, func(caps []string) bool {
		out = append(out, caps)
		return limit <= 0 || len(out) < limit
	})
	return out
}

// FindLast returns the final n matches, in document order. It is used for
// pages that list oldest entries first, where the newest items are at the
// bottom: taking the first n matches there would permanently hide new items.
// Memory stays bounded by n regardless of how many matches the page holds.
func (p *Pattern) FindLast(page string, n int) [][]string {
	if n <= 0 {
		return p.FindAll(page, 0)
	}
	ring := make([][]string, n)
	count := 0
	p.each(page, func(caps []string) bool {
		ring[count%n] = caps
		count++
		return true
	})
	if count < n {
		return ring[:count]
	}
	out := make([][]string, 0, n)
	for i := count; i < count+n; i++ {
		out = append(out, ring[i%n])
	}
	return out
}

// each walks non-overlapping matches left to right, invoking fn with each
// match's captures. Iteration stops when fn returns false.
func (p *Pattern) each(page string, fn func(caps []string) bool) {
	nSub := p.re.NumSubexp()
	for pos := 0; pos <= len(page); {
		loc := p.re.FindStringSubmatchIndex(page[pos:])
		if loc == nil {
			return
		}
		start, end := pos+loc[0], pos+loc[1]
		if end == start {
			// Zero-width match: advance one rune to guarantee progress.
			_, w := utf8.DecodeRuneInString(page[start:])
			if w < 1 {
				w = 1
			}
			pos = start + w
			continue
		}
		caps := make([]string, nSub)
		for i := 1; i <= nSub; i++ {
			if s, e := loc[2*i], loc[2*i+1]; s >= 0 {
				caps[i-1] = page[pos+s : pos+e]
			}
		}
		if !fn(caps) {
			return
		}
		pos = end
	}
}

var placeholderRe = regexp.MustCompile(`\{%(\d{1,2})\}`)

// Render substitutes {%1}, {%2}, … in tmpl with the corresponding captures
// (1-based). Placeholders without a matching capture become empty strings.
func Render(tmpl string, captures []string) string {
	if tmpl == "" {
		return ""
	}
	return placeholderRe.ReplaceAllStringFunc(tmpl, func(ph string) string {
		n, err := strconv.Atoi(placeholderRe.FindStringSubmatch(ph)[1])
		if err != nil || n < 1 || n > len(captures) {
			return ""
		}
		return captures[n-1]
	})
}
