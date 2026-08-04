package clean

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata expected files from current Clean() output")

func TestClean(t *testing.T) {
	cases := []struct {
		name string
		in   string
		opts Opts
		want string
	}{
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "whitespace only collapses to single newline",
			in:   "   \n  \t  \n",
			want: "\n",
		},
		{
			name: "single line no newline",
			in:   "hello world",
			want: "hello world",
		},
		{
			name: "trailing whitespace stripped",
			in:   "hello   \nworld\t\t\n",
			want: "hello\nworld\n",
		},
		{
			name: "uniform leading whitespace dedented",
			in:   "  hello\n    code\n  text",
			want: "hello\n  code\ntext",
		},
		{
			name: "leading border char stripped",
			in:   "│ first\n│ second\n│ third",
			want: "first\nsecond\nthird",
		},
		{
			name: "leading pipe variant stripped",
			in:   "| one\n| two\n| three",
			want: "one\ntwo\nthree",
		},
		{
			name: "leading whitespace then border char",
			in:   "  │ a\n  │ b\n  │ c",
			want: "a\nb\nc",
		},
		{
			name: "claude code quote bar stripped",
			in:   "▎ first\n▎ second\n▎ third",
			want: "first\nsecond\nthird",
		},
		{
			name: "thin quote bar stripped",
			in:   "▏ first\n▏ second\n▏ third",
			want: "first\nsecond\nthird",
		},
		{
			name: "fully bordered output",
			in:   "│ hello   │\n│ world   │\n│ done    │",
			want: "hello\nworld\ndone",
		},
		{
			name: "nested borders",
			in:   "│ │ a │ │\n│ │ b │ │\n│ │ c │ │",
			want: "a\nb\nc",
		},
		{
			name: "mid-line border char preserved",
			in:   "use the | pipe operator\nto chain | commands\nin your | shell",
			want: "use the | pipe operator\nto chain | commands\nin your | shell",
		},
		{
			name: "code block indentation preserved after dedent",
			in:   "  prose line\n      indented code\n      more code\n  back to prose",
			want: "prose line\n    indented code\n    more code\nback to prose",
		},
		{
			name: "blank lines preserved",
			in:   "│ first paragraph\n│\n│ second paragraph",
			want: "first paragraph\n\nsecond paragraph",
		},
		{
			name: "ANSI off by default leaves codes",
			in:   "\x1b[31mred\x1b[0m text",
			want: "\x1b[31mred\x1b[0m text",
		},
		{
			name: "ANSI on strips codes",
			in:   "\x1b[31mred\x1b[0m text",
			opts: Opts{StripANSI: true},
			want: "red text",
		},
		{
			name: "OSC hyperlink stripped when enabled",
			in:   "\x1b]8;;https://x.com\x07link\x1b]8;;\x07",
			opts: Opts{StripANSI: true},
			want: "link",
		},
		{
			name: "list items not rejoined",
			in:   "- first item\n- second item\n- third item",
			want: "- first item\n- second item\n- third item",
		},
		{
			name: "rejoin wrapped prose at terminal width",
			in: "this is a long line of prose that fills the terminal width\n" +
				"continuation of that same sentence here.\n" +
				"this is a long line of prose that fills the terminal width\n" +
				"continuation of that same sentence here.",
			want: "this is a long line of prose that fills the terminal width continuation of that same sentence here.\n" +
				"this is a long line of prose that fills the terminal width continuation of that same sentence here.",
		},
		{
			name: "no rejoin when disabled",
			in: "this is a long line of prose that fills the terminal width\n" +
				"continuation of that same sentence here.",
			opts: Opts{NoRejoin: true},
			want: "this is a long line of prose that fills the terminal width\n" +
				"continuation of that same sentence here.",
		},
		{
			name: "rejoin skips fenced code blocks",
			in:   "intro line\n```\nlong code line that almost reaches terminal width here\nshort\n```\noutro line",
			want: "intro line\n```\nlong code line that almost reaches terminal width here\nshort\n```\noutro line",
		},
		{
			name: "headings not rejoined into",
			in: "this line is fairly long and reaches near terminal width here\n" +
				"# Heading should not merge",
			want: "this line is fairly long and reaches near terminal width here\n" +
				"# Heading should not merge",
		},
		{
			name: "capital starting next line not rejoined",
			in: "this line is fairly long and reaches near terminal width here\n" +
				"Next sentence starts capitalized.",
			want: "this line is fairly long and reaches near terminal width here\n" +
				"Next sentence starts capitalized.",
		},
		{
			name: "triple+ blank runs collapsed",
			in:   "a\n\n\n\n\nb",
			want: "a\n\n\nb", // collapseBlankRuns keeps 2 blanks → 3 newlines between non-blanks
		},
		{
			name: "mixed prefix and non-prefix below threshold leaves all alone",
			in:   "│ a\nplain b\nplain c\nplain d\nplain e",
			want: "│ a\nplain b\nplain c\nplain d\nplain e",
		},
		{
			name: "prefix above threshold strips matching, leaves outliers",
			in:   "│ a\n│ b\n│ c\n│ d\nplain e",
			want: "a\nb\nc\nd\nplain e",
		},
		// --- 1a: all border char variants ---
		{
			name: "heavy vertical border char stripped",
			in:   "┃ first\n┃ second\n┃ third",
			want: "first\nsecond\nthird",
		},
		{
			name: "gt border char stripped",
			in:   "> first\n> second\n> third",
			want: "first\nsecond\nthird",
		},
		{
			name: "light dashed vertical border char stripped",
			in:   "┆ first\n┆ second\n┆ third",
			want: "first\nsecond\nthird",
		},
		{
			name: "light triple-dash vertical border char stripped",
			in:   "╎ first\n╎ second\n╎ third",
			want: "first\nsecond\nthird",
		},
		{
			name: "light dotted vertical border char stripped",
			in:   "┊ first\n┊ second\n┊ third",
			want: "first\nsecond\nthird",
		},
		{
			name: "heavy dotted vertical border char stripped",
			in:   "┇ first\n┇ second\n┇ third",
			want: "first\nsecond\nthird",
		},
		{
			name: "heavy quad-dash vertical border char stripped",
			in:   "╏ first\n╏ second\n╏ third",
			want: "first\nsecond\nthird",
		},
		{
			name: "left half block border char stripped",
			in:   "▌ first\n▌ second\n▌ third",
			want: "first\nsecond\nthird",
		},
		// --- 1b: rejoin list-marker variants ---
		{
			name: "star bullet list items not rejoined",
			in:   "* this is a fairly long list item that approaches terminal width here\n* and here is another list item",
			want: "* this is a fairly long list item that approaches terminal width here\n* and here is another list item",
		},
		{
			name: "plus bullet list items not rejoined",
			in:   "+ this is a fairly long list item that approaches terminal width here\n+ and here is another list item",
			want: "+ this is a fairly long list item that approaches terminal width here\n+ and here is another list item",
		},
		{
			name: "numbered list items not rejoined",
			in:   "1. this is a fairly long list item that approaches terminal width here\n2. and here is another list item",
			want: "1. this is a fairly long list item that approaches terminal width here\n2. and here is another list item",
		},
		{
			name: "quote block lines not rejoined",
			in:   "intro\n> first quoted line is fairly long and approaches terminal width here\n> second quoted line",
			want: "intro\n> first quoted line is fairly long and approaches terminal width here\n> second quoted line",
		},
		// --- 1b1: Unicode bullet / symbol-prefixed continuations ---
		{
			name: "unicode bullet continuation not rejoined",
			in: "this is a long line of prose that fills the terminal width\n" +
				"⏺ Read foo.txt",
			want: "this is a long line of prose that fills the terminal width\n" +
				"⏺ Read foo.txt",
		},
		{
			name: "emoji checkmark continuation not rejoined",
			in: "this is a long line of prose that fills the terminal width\n" +
				"✅ Tests passed",
			want: "this is a long line of prose that fills the terminal width\n" +
				"✅ Tests passed",
		},
		{
			name: "emoji pushpin continuation not rejoined",
			in: "this is a long line of prose that fills the terminal width\n" +
				"📌 Important note here",
			want: "this is a long line of prose that fills the terminal width\n" +
				"📌 Important note here",
		},
		{
			name: "non-list pictograph continuation guarded by So",
			in: "this is a long line of prose that fills the terminal width\n" +
				"★ a starred item",
			want: "this is a long line of prose that fills the terminal width\n" +
				"★ a starred item",
		},
		// --- 1c: fenced code edge cases ---
		{
			name: "fenced code with language tag not rejoined inside",
			in:   "intro line\n```go\nlong code line that almost reaches terminal width here\nshort\n```\noutro line",
			want: "intro line\n```go\nlong code line that almost reaches terminal width here\nshort\n```\noutro line",
		},
		{
			name: "tilde fence skips rejoin inside",
			in:   "intro line\n~~~\nlong code line that almost reaches terminal width here\nshort\n~~~\noutro line",
			want: "intro line\n~~~\nlong code line that almost reaches terminal width here\nshort\n~~~\noutro line",
		},
		{
			name: "unclosed fence suppresses rejoin to end of document",
			in:   "intro line\n```\nlong code line that almost reaches terminal width here\ncontinuation",
			want: "intro line\n```\nlong code line that almost reaches terminal width here\ncontinuation",
		},
		// --- 1d: threshold boundary ---
		{
			name: "prefix at 75 percent below threshold not stripped",
			in:   "│ a\n│ b\n│ c\nplain d",
			want: "│ a\n│ b\n│ c\nplain d",
		},
		// --- 1e: tab-based indentation ---
		{
			name: "uniform leading tab whitespace dedented",
			in:   "\thello\n\t\tcode\n\ttext",
			want: "hello\n\tcode\ntext",
		},
		// --- 1f: CRLF normalization ---
		{
			name: "CRLF line endings normalized",
			in:   "hello\r\nworld\r\n",
			want: "hello\nworld\n",
		},
		{
			name: "whitespace-only blank line does not block leading border detection",
			in:   "│ a\n   \n│ b\n│ c",
			want: "a\n\nb\nc",
		},
		// --- 2a: markdown table guard (leading and trailing '|' preserved) ---
		{
			name: "markdown table left border preserved",
			in:   "| col1 | col2 |\n| ---- | ---- |\n| a    | b    |\n| c    | d    |",
			want: "| col1 | col2 |\n| ---- | ---- |\n| a    | b    |\n| c    | d    |",
		},
		{
			name: "markdown table without trailing pipes preserved",
			in:   "| col1 | col2\n| ---- | ----\n| a    | b\n| c    | d",
			want: "| col1 | col2\n| ---- | ----\n| a    | b\n| c    | d",
		},
		// --- 2b: full-box borders (horizontal lines stripped, contents kept) ---
		{
			name: "full-box top and bottom horizontal rules removed",
			in:   "┌─────┐\n│ hi  │\n└─────┘",
			want: "hi",
		},
		{
			name: "double-line horizontal rule removed",
			in:   "═════════\nstuff goes here\n═════════",
			want: "stuff goes here",
		},
		{
			name: "ascii dashes treated as content not chrome",
			in:   "alpha\n---\nbeta",
			want: "alpha\n---\nbeta",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := Clean(tc.in, tc.opts)
			if got != tc.want {
				t.Errorf("Clean mismatch\n--- input ---\n%q\n--- want ---\n%q\n--- got ---\n%q", tc.in, tc.want, got)
			}
		})
	}
}

func TestCleanStats(t *testing.T) {
	type want struct {
		leadingChar          rune
		leadingLines         int
		trailingChar         rune
		trailingLines        int
		boxBorderLinesRemove int
		rejoinedLines        int
		blankRunsCollapsed   int
		dedentColumnsAtLeast int
		leadingCapHit        bool
		trailingCapHit       bool
		unclosedFence        bool
		markdownTableSkipped int
	}

	cases := []struct {
		name string
		in   string
		opts Opts
		want want
	}{
		{
			name: "leading border counted",
			in:   "│ a\n│ b\n│ c",
			want: want{leadingChar: '│', leadingLines: 3},
		},
		{
			name: "trailing border counted",
			in:   "a   │\nb   │\nc   │",
			want: want{trailingChar: '│', trailingLines: 3},
		},
		{
			name: "box border lines counted",
			in:   "┌─────┐\n│ hi  │\n└─────┘",
			want: want{
				boxBorderLinesRemove: 2,
				leadingChar:          '│',
				leadingLines:         1,
				trailingChar:         '│',
				trailingLines:        1,
			},
		},
		{
			name: "rejoin counter increments",
			in: "this is a long line of prose that fills the terminal width\n" +
				"continuation of that same sentence here.\n" +
				"this is a long line of prose that fills the terminal width\n" +
				"continuation of that same sentence here.",
			want: want{rejoinedLines: 2},
		},
		{
			name: "blank-run collapse counter",
			in:   "a\n\n\n\n\nb\n\n\n\nc",
			want: want{blankRunsCollapsed: 2},
		},
		{
			name: "unclosed fence flagged",
			in: "intro line that is fairly long and approaches terminal width here\n" +
				"```\nlong code line that almost reaches the terminal width here too\ntail",
			want: want{unclosedFence: true},
		},
		{
			name: "markdown table skip flagged",
			in:   "| col1 | col2 |\n| ---- | ---- |\n| a    | b    |\n| c    | d    |",
			want: want{markdownTableSkipped: 2}, // both leading and trailing guard fire
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, got := Clean(tc.in, tc.opts)
			if got.LeadingBorderChar != tc.want.leadingChar {
				t.Errorf("LeadingBorderChar: got %q want %q", got.LeadingBorderChar, tc.want.leadingChar)
			}
			if got.LeadingBorderLines != tc.want.leadingLines {
				t.Errorf("LeadingBorderLines: got %d want %d", got.LeadingBorderLines, tc.want.leadingLines)
			}
			if got.TrailingBorderChar != tc.want.trailingChar {
				t.Errorf("TrailingBorderChar: got %q want %q", got.TrailingBorderChar, tc.want.trailingChar)
			}
			if got.TrailingBorderLines != tc.want.trailingLines {
				t.Errorf("TrailingBorderLines: got %d want %d", got.TrailingBorderLines, tc.want.trailingLines)
			}
			if got.BoxBorderLinesRemoved != tc.want.boxBorderLinesRemove {
				t.Errorf("BoxBorderLinesRemoved: got %d want %d", got.BoxBorderLinesRemoved, tc.want.boxBorderLinesRemove)
			}
			if got.RejoinedLines != tc.want.rejoinedLines {
				t.Errorf("RejoinedLines: got %d want %d", got.RejoinedLines, tc.want.rejoinedLines)
			}
			if got.BlankRunsCollapsed != tc.want.blankRunsCollapsed {
				t.Errorf("BlankRunsCollapsed: got %d want %d", got.BlankRunsCollapsed, tc.want.blankRunsCollapsed)
			}
			if tc.want.dedentColumnsAtLeast > 0 && got.DedentColumns < tc.want.dedentColumnsAtLeast {
				t.Errorf("DedentColumns: got %d want at least %d", got.DedentColumns, tc.want.dedentColumnsAtLeast)
			}
			if got.LeadingCapHit != tc.want.leadingCapHit {
				t.Errorf("LeadingCapHit: got %v want %v", got.LeadingCapHit, tc.want.leadingCapHit)
			}
			if got.TrailingCapHit != tc.want.trailingCapHit {
				t.Errorf("TrailingCapHit: got %v want %v", got.TrailingCapHit, tc.want.trailingCapHit)
			}
			if got.UnclosedFence != tc.want.unclosedFence {
				t.Errorf("UnclosedFence: got %v want %v", got.UnclosedFence, tc.want.unclosedFence)
			}
			if got.MarkdownTableSkipped != tc.want.markdownTableSkipped {
				t.Errorf("MarkdownTableSkipped: got %d want %d", got.MarkdownTableSkipped, tc.want.markdownTableSkipped)
			}
		})
	}
}

func TestCleanIdempotentOverTestdata(t *testing.T) {
	dir := "../../testdata/examples"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("testdata/examples dir not present: %v", err)
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, "_sample.txt") {
			continue
		}
		t.Run(strings.TrimSuffix(name, "_sample.txt"), func(t *testing.T) {
			in, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			first, _ := Clean(string(in), Opts{})
			second, _ := Clean(first, Opts{})
			if first != second {
				t.Errorf("Clean is not idempotent on %s\n--- first ---\n%s\n--- second ---\n%s", name, first, second)
			}
		})
	}
}

func TestCleanFromTestdata(t *testing.T) {
	dir := "../../testdata/examples"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("testdata/examples dir not present: %v", err)
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, "_sample.txt") {
			continue
		}
		base := strings.TrimSuffix(name, "_sample.txt")
		expectedPath := filepath.Join(dir, base+"_expected.txt")
		t.Run(base, func(t *testing.T) {
			in, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			got, _ := Clean(string(in), Opts{})
			want, err := os.ReadFile(expectedPath)
			if err != nil {
				if os.IsNotExist(err) && *update {
					if werr := os.WriteFile(expectedPath, []byte(got), 0644); werr != nil {
						t.Fatalf("write expected: %v", werr)
					}
					t.Logf("wrote %s", expectedPath)
					return
				}
				if os.IsNotExist(err) {
					t.Fatalf("missing expected file %q — run `go test ./internal/clean -update` to generate it", expectedPath)
				}
				t.Fatal(err)
			}
			if got != string(want) {
				if *update {
					if werr := os.WriteFile(expectedPath, []byte(got), 0644); werr != nil {
						t.Fatalf("rewrite expected: %v", werr)
					}
					t.Logf("rewrote %s", expectedPath)
					return
				}
				t.Errorf("mismatch on %s\n--- want ---\n%s\n--- got ---\n%s", base, string(want), got)
			}
		})
	}
}
