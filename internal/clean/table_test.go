package clean

import (
	"strings"
	"testing"
)

// rawBoxTable is a box-drawing render with the two properties that make
// this hard: cells the terminal wrapped across several physical lines,
// and a row whose wrap leaves other columns blank on the continuation
// line — the case that is unrecoverable once the borders are gone.
const rawBoxTable = `Comparing the three build modes:

┌──────────────┬───────────────────────────────────────────────────────────────────────────────────────────┬───────────────────────────────┐
│     Mode     │                                            Behaviour                                               │            Best for             │
├──────────────┼───────────────────────────────────────────────────────────────────────────────────────────┼───────────────────────────────┤
│ debug        │ no optimisation, full symbol tables, assertions left in, rebuilds only what changed                │ day-to-day work on a single     │
│              │                                                                                                    │ package                         │
├──────────────┼───────────────────────────────────────────────────────────────────────────────────────────┼───────────────────────────────┤
│ release      │ full optimisation, symbols stripped, assertions compiled out                                       │ anything you ship               │
├──────────────┼───────────────────────────────────────────────────────────────────────────────────────────┼───────────────────────────────┤
│ profile      │ optimised like release but keeps frame pointers and symbols so a profiler can walk the stack       │ finding out where the time      │
│              │                                                                                                    │ actually goes                   │
└──────────────┴───────────────────────────────────────────────────────────────────────────────────────────┴───────────────────────────────┘`

func TestCleanRebuildsRawBoxTable(t *testing.T) {
	got, stats := Clean(rawBoxTable, Opts{})

	if stats.BoxTablesRebuilt != 1 {
		t.Errorf("BoxTablesRebuilt = %d, want 1", stats.BoxTablesRebuilt)
	}

	if !strings.HasPrefix(got, "Comparing the three build modes:") {
		t.Fatalf("leading non-table text was not preserved:\n%s", got)
	}

	want := []string{
		"| Mode | Behaviour | Best for |",
		"| --- | --- | --- |",
		"| debug | no optimisation, full symbol tables, assertions left in, rebuilds only what changed | day-to-day work on a single package |",
		"| release | full optimisation, symbols stripped, assertions compiled out | anything you ship |",
		"| profile | optimised like release but keeps frame pointers and symbols so a profiler can walk the stack | finding out where the time actually goes |",
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Fatalf("missing row %q in:\n%s", w, got)
		}
	}

	for _, box := range []string{"┌", "│", "└", "├"} {
		if strings.Contains(got, box) {
			t.Fatalf("box-drawing character %q survived reconstruction:\n%s", box, got)
		}
	}
}

func TestRebuildBoxTableSimpleTwoRows(t *testing.T) {
	got, _ := Clean("┌───┬───┐\n│ A │ B │\n├───┼───┤\n│ C │ D │\n└───┴───┘", Opts{})
	want := "| A | B |\n| --- | --- |\n| C | D |"
	if got != want {
		t.Fatalf("Clean() = %q, want %q", got, want)
	}
}

func TestRebuildBoxTableEscapesLiteralPipes(t *testing.T) {
	got, _ := Clean("┌──────┬───┐\n│ a|b  │ B │\n├──────┼───┤\n│ C    │ D │\n└──────┴───┘", Opts{})
	if !strings.Contains(got, `a\|b`) {
		t.Fatalf("literal pipe in cell content was not escaped:\n%s", got)
	}
}

func TestRebuildBoxTableLeavesUnterminatedBoxAlone(t *testing.T) {
	in := "Before\n┌────┬────┐\nno closing border, not a real table\nAfter"
	got := rebuildBoxTables(strings.Split(in, "\n"), nil)
	if strings.Join(got, "\n") != in {
		t.Fatalf("rebuildBoxTables() = %q, want unchanged %q", strings.Join(got, "\n"), in)
	}
}

func TestRebuildBoxTableLeavesNonTableTextAlone(t *testing.T) {
	in := "Just a normal paragraph.\n\nAnother line, no box drawing here at all."
	got := rebuildBoxTables(strings.Split(in, "\n"), nil)
	if strings.Join(got, "\n") != in {
		t.Fatalf("rebuildBoxTables() = %q, want unchanged %q", strings.Join(got, "\n"), in)
	}
}

// A single framed row is a panel, not a table; rebuilding it as a
// header-only markdown table renders worse than the border-stripping
// stages already do.
func TestRebuildBoxTableLeavesSingleRowPanelAlone(t *testing.T) {
	in := "┌───────┬───────┐\n│ alpha │ beta  │\n└───────┴───────┘"
	got := rebuildBoxTables(strings.Split(in, "\n"), nil)
	if strings.Join(got, "\n") != in {
		t.Fatalf("rebuildBoxTables() = %q, want unchanged %q", strings.Join(got, "\n"), in)
	}
}

func TestCleanRebuiltTableSurvivesSecondPass(t *testing.T) {
	once, _ := Clean(rawBoxTable, Opts{})
	twice, stats := Clean(once, Opts{})
	if once != twice {
		t.Errorf("Clean is not idempotent over a rebuilt table\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}
	if stats.BoxTablesRebuilt != 0 {
		t.Errorf("second pass rebuilt %d table(s), want 0", stats.BoxTablesRebuilt)
	}
}
