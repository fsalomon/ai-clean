// Package clean normalizes text copied from AI-CLI terminal output:
// strips terminal chrome (borders, padding), trims whitespace, and
// optionally rejoins lines that the terminal hard-wrapped.
package clean

import "strings"

// Opts controls the cleanup pipeline.
type Opts struct {
	// StripANSI removes ANSI / OSC escape sequences. Off by default
	// because most terminals already strip these on copy; turning it
	// on keeps surviving codes from leaking through.
	StripANSI bool

	// NoRejoin disables the wrapped-line rejoin heuristic. Useful when
	// pasting pure code where any reflow is unwanted.
	NoRejoin bool
}

// Stats reports what each stage of the pipeline did. Powers --explain
// in the CLI and is useful for debugging heuristic changes. All fields
// are zero-valued when the corresponding stage made no change.
type Stats struct {
	LeadingBorderChar     rune
	LeadingBorderLines    int
	TrailingBorderChar    rune
	TrailingBorderLines   int
	DedentColumns         int
	BoxBorderLinesRemoved int
	BoxTablesRebuilt      int
	RejoinedLines         int
	BlankRunsCollapsed    int
	LeadingCapHit         bool
	TrailingCapHit        bool
	UnclosedFence         bool
	MarkdownTableSkipped  int
}

// Clean runs the full cleanup pipeline on text and returns the result
// plus a Stats record describing what changed. Order is fixed; see the
// package doc and the project plan for rationale.
func Clean(text string, opts Opts) (string, Stats) {
	var stats Stats

	if opts.StripANSI {
		text = stripANSI(text)
	}

	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")

	// Box tables are rebuilt once, before the loop, because the stage is
	// destructive-by-design: it consumes the rule lines that define row
	// boundaries. Running it inside the fix-point loop would let a table
	// that only *appears* after an outer border is stripped get rebuilt on
	// a later pass, but it would also risk re-parsing cell content that
	// happens to look like a rule line, which breaks idempotency. Once,
	// on the raw input, is the safe half of that trade — and by the time
	// the loop finishes, stripFullBoxBorders has removed every rule line
	// anyway, so a second Clean() has nothing left to rebuild.
	lines = rebuildBoxTables(lines, &stats)

	// Fix-point: each stage can produce input that another stage would
	// clean further (trailing-strip can empty a line, changing dedent
	// thresholds; trailing-strip can also leave a line that's now pure
	// box-drawing content; rejoin can expose leading whitespace from the
	// merged tail; etc.). Loop until the whole pipeline converges.
	// Convergence is bounded — each changing pass strictly shrinks the
	// document (removes at least one rune of border, whitespace, or
	// merges at least one newline).
	for i := 0; i < pipelineSafetyCap; i++ {
		before := strings.Join(lines, "\n")
		lines = stripFullBoxBorders(lines, &stats)
		lines = stripLeadingChrome(lines, &stats)
		lines = stripTrailingChrome(lines, &stats)
		if !opts.NoRejoin {
			lines = rejoinWrapped(lines, &stats)
		}
		if strings.Join(lines, "\n") == before {
			break
		}
	}

	lines = collapseBlankRuns(lines, &stats)

	return strings.Join(lines, "\n"), stats
}

// collapseBlankRuns reduces any run of 3+ blank lines down to 2.
// Long blank runs usually come from bordered output where every "blank"
// row was full of padding; once stripped, they collapse into many empties.
func collapseBlankRuns(lines []string, stats *Stats) []string {
	out := make([]string, 0, len(lines))
	blanks := 0
	for _, l := range lines {
		if l == "" {
			blanks++
			if blanks <= 2 {
				out = append(out, l)
			}
			continue
		}
		if blanks > 2 && stats != nil {
			stats.BlankRunsCollapsed++
		}
		blanks = 0
		out = append(out, l)
	}
	if blanks > 2 && stats != nil {
		stats.BlankRunsCollapsed++
	}
	return out
}
