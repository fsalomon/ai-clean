package clean

import "strings"

// borderChars are the candidate left/right border characters used by
// CLIs to draw boxed output. Order doesn't matter; each is checked
// independently against the threshold.
//
// The block-element run U+2589..U+258F (▉▊▋▌▍▎▏) is listed in full
// because CLIs pick arbitrary widths out of it for quote bars, and the
// glyphs are near-indistinguishable at terminal font sizes — Claude
// Code's quoted blocks use ▎ (U+258E), not the ▌ (U+258C) that looks
// like the obvious candidate. U+2588 (█) is deliberately left out: a
// full block is more often graphics fill or a progress bar than chrome.
var borderChars = []rune{'│', '┃', '|', '>', '┆', '╎', '┊', '┇', '╏', '▉', '▊', '▋', '▌', '▍', '▎', '▏'}

// borderThreshold: a candidate is accepted as a uniform border when
// it appears at the relevant position on at least this fraction of
// non-empty lines. 0.8 tolerates the occasional missing-border line
// while still rejecting characters that just happen to appear
// frequently in normal text.
const borderThreshold = 0.8

// markdownTableThreshold: when the candidate border is '|', skip
// stripping if at least this fraction of border-having lines also
// carry an interior '|' — that pattern is a markdown table, not a
// CLI border. Lower than borderThreshold because table separators
// (`|---|---|`) and continuation rows can dilute the interior count.
const markdownTableThreshold = 0.5

// nestingWarnThreshold: number of dedent+border-strip passes beyond
// which we record a "deep nesting" warning in Stats. Real-world chrome
// rarely nests deeper than 2; >3 is unusual enough to flag.
const nestingWarnThreshold = 3

// pipelineSafetyCap: hard upper bound on loop iterations. Each pass
// that changes anything strictly shrinks the document (it strips at
// least one rune of border or whitespace), so convergence is guaranteed
// in O(input size). This bound exists only to make a heuristic bug fail
// loudly instead of looping forever.
const pipelineSafetyCap = 100

// stripLeadingChrome removes uniform leading whitespace and uniform
// leading border characters in alternating passes until a full pass
// produces no change. If more than nestingWarnThreshold passes were
// needed, stats.LeadingCapHit is set so --explain can surface it.
//
// Whitespace-dedent runs first within each pass: the minimum leading
// whitespace count across non-empty lines is computed and stripped from
// every non-empty line, preserving relative indentation of code blocks.
func stripLeadingChrome(lines []string, stats *Stats) []string {
	passes := 0
	for passes < pipelineSafetyCap {
		before := joinForCompare(lines)
		lines = dedentLeadingWhitespace(lines, stats)
		lines = stripLeadingBorderChar(lines, stats)
		if joinForCompare(lines) == before {
			break
		}
		passes++
	}
	if stats != nil && passes > nestingWarnThreshold {
		stats.LeadingCapHit = true
	}
	return lines
}

// dedentLeadingWhitespace strips a uniform leading-whitespace pad. Uses the
// same ≥80% threshold as border detection so a single outlier line at column
// 0 (common at the top of pasted summaries) can't block the dedent.
func dedentLeadingWhitespace(lines []string, stats *Stats) []string {
	counts := make([]int, len(lines))
	considered := 0
	for i, l := range lines {
		if l == "" {
			counts[i] = -1
			continue
		}
		n := 0
		for _, r := range l {
			if r == ' ' || r == '\t' {
				n++
				continue
			}
			break
		}
		if n == len(l) {
			counts[i] = -1
			continue
		}
		counts[i] = n
		considered++
	}
	if considered == 0 {
		return lines
	}

	threshold := int(float64(considered)*borderThreshold + 0.5)
	if threshold < 1 {
		threshold = 1
	}
	cut := 0
	for n := 1; ; n++ {
		c := 0
		for _, k := range counts {
			if k >= n {
				c++
			}
		}
		if c < threshold {
			break
		}
		cut = n
	}
	if cut == 0 {
		return lines
	}

	out := make([]string, len(lines))
	for i, l := range lines {
		if l == "" {
			out[i] = l
			continue
		}
		k := cut
		if counts[i] >= 0 && counts[i] < k {
			k = counts[i]
		}
		if k > len(l) {
			k = len(l)
		}
		safe := true
		for j := 0; j < k; j++ {
			if l[j] != ' ' && l[j] != '\t' {
				safe = false
				break
			}
		}
		if safe {
			out[i] = l[k:]
		} else {
			out[i] = l
		}
	}
	if stats != nil {
		stats.DedentColumns += cut
	}
	return out
}

func stripLeadingBorderChar(lines []string, stats *Stats) []string {
	nonEmpty := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		return lines
	}

	type cand struct {
		ch    rune
		count int
	}
	var best cand
	for _, ch := range borderChars {
		c := 0
		for _, l := range lines {
			if l == "" {
				continue
			}
			rs := []rune(l)
			if len(rs) > 0 && rs[0] == ch {
				c++
			}
		}
		if c > best.count {
			best = cand{ch: ch, count: c}
		}
	}

	if float64(best.count)/float64(nonEmpty) < borderThreshold {
		return lines
	}

	if best.ch == '|' && looksLikeMarkdownTable(lines) {
		if stats != nil {
			stats.MarkdownTableSkipped++
		}
		return lines
	}

	out := make([]string, len(lines))
	for i, l := range lines {
		if l == "" {
			out[i] = l
			continue
		}
		rs := []rune(l)
		if len(rs) == 0 || rs[0] != best.ch {
			out[i] = l
			continue
		}
		// Drop the border char and one optional following space.
		rs = rs[1:]
		if len(rs) > 0 && rs[0] == ' ' {
			rs = rs[1:]
		}
		out[i] = string(rs)
	}
	if stats != nil {
		if stats.LeadingBorderChar == 0 {
			stats.LeadingBorderChar = best.ch
		}
		stats.LeadingBorderLines += best.count
	}
	return out
}

// looksLikeMarkdownTable returns true when the lines starting with '|'
// also have at least one interior '|' on a sufficient fraction of those
// rows — i.e. the leading '|' is a table column delimiter, not a border.
func looksLikeMarkdownTable(lines []string) bool {
	leading := 0
	withInterior := 0
	for _, l := range lines {
		rs := []rune(strings.TrimRight(l, " \t"))
		if len(rs) == 0 || rs[0] != '|' {
			continue
		}
		leading++
		// Drop the trailing '|' if present (cell-closing pipe of a table row).
		end := len(rs)
		if rs[end-1] == '|' {
			end--
		}
		for j := 1; j < end; j++ {
			if rs[j] == '|' {
				withInterior++
				break
			}
		}
	}
	if leading == 0 {
		return false
	}
	return float64(withInterior)/float64(leading) >= markdownTableThreshold
}

func joinForCompare(lines []string) string {
	return strings.Join(lines, "\n")
}
