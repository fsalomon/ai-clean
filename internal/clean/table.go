package clean

import "strings"

// rebuildBoxTables converts terminal box-drawing tables — the render AI
// CLIs produce for a Markdown table — back into Markdown table syntax.
//
// Without this, such a table is the one input the rest of the pipeline
// cannot help: stripFullBoxBorders deletes the ┌─┬─┐ rule lines, then
// stripLeadingChrome sees a uniform '│' and strips it, and what's left
// is column fragments with no structure and no way to tell wrapped
// continuation lines from new rows. The Markdown-table guard in
// leading.go only protects tables that are *already* '|'-shaped, so the
// reconstruction has to happen before anything else touches the lines.
//
// The rule lines are what make this a structural parse rather than a
// heuristic: every content line between two of them belongs to the same
// logical row, however many of its cells are blank. Detecting wrapped
// cells after the borders are gone would be guesswork.
func rebuildBoxTables(lines []string, stats *Stats) []string {
	out := make([]string, 0, len(lines))
	rebuilt := 0

	for i := 0; i < len(lines); {
		if isTableTopBorder(lines[i]) {
			table, consumed := parseBoxTable(lines[i:])
			if table != nil {
				out = append(out, table...)
				i += consumed
				rebuilt++
				continue
			}
		}
		out = append(out, lines[i])
		i++
	}

	if stats != nil {
		stats.BoxTablesRebuilt += rebuilt
	}
	return out
}

func isTableTopBorder(line string) bool {
	return isTableRuleLine(line, '┌', '┬', '┐')
}

func isTableRowSeparator(line string) bool {
	return isTableRuleLine(line, '├', '┼', '┤')
}

func isTableBottomBorder(line string) bool {
	return isTableRuleLine(line, '└', '┴', '┘')
}

// isTableRuleLine accepts a single-column rule (no mid junction) as well,
// so a one-column table still parses instead of falling through to the
// border-stripping path that would flatten it.
func isTableRuleLine(line string, left, mid, right rune) bool {
	rs := []rune(strings.TrimSpace(line))
	if len(rs) < 2 || rs[0] != left || rs[len(rs)-1] != right {
		return false
	}
	for _, r := range rs[1 : len(rs)-1] {
		if r != '─' && r != mid {
			return false
		}
	}
	return true
}

// parseBoxTable expects lines[0] to be a top border. It returns the
// reconstructed Markdown lines and how many input lines it consumed, or
// (nil, 0) when the structure past the top border isn't a well-formed
// table — the caller then leaves the line alone for the other stages.
func parseBoxTable(lines []string) ([]string, int) {
	i := 1
	var rowLines [][]string
	var current []string

	for i < len(lines) {
		line := lines[i]
		switch {
		case isTableBottomBorder(line):
			if len(current) > 0 {
				rowLines = append(rowLines, current)
			}
			i++
			table := renderMarkdownTable(rowLines)
			if table == nil {
				return nil, 0
			}
			return table, i
		case isTableRowSeparator(line):
			if len(current) > 0 {
				rowLines = append(rowLines, current)
				current = nil
			}
			i++
		case strings.Contains(line, "│"):
			current = append(current, line)
			i++
		default:
			return nil, 0
		}
	}

	return nil, 0
}

// renderMarkdownTable needs at least a header row and one body row; a
// lone row is more likely a framed panel than a table, and turning it
// into a header-only Markdown table would be a worse render than what
// the ordinary border-stripping stages produce.
func renderMarkdownTable(rowLines [][]string) []string {
	if len(rowLines) < 2 {
		return nil
	}

	rows := make([][]string, len(rowLines))
	for i, physicalLines := range rowLines {
		rows[i] = mergeWrappedRow(physicalLines)
	}

	colCount := len(rows[0])
	if colCount == 0 {
		return nil
	}

	out := make([]string, 0, len(rows)+1)
	out = append(out, renderTableRow(rows[0]))
	out = append(out, renderTableSeparator(colCount))
	for _, row := range rows[1:] {
		out = append(out, renderTableRow(row))
	}
	return out
}

// mergeWrappedRow joins the cells of every physical line belonging to one
// logical row, column by column, so a cell the terminal wrapped across
// three lines comes back as one cell.
func mergeWrappedRow(physicalLines []string) []string {
	var merged []string
	for _, line := range physicalLines {
		cells := splitTableCells(line)
		if merged == nil {
			merged = cells
			continue
		}
		for i, c := range cells {
			if i >= len(merged) {
				merged = append(merged, c)
				continue
			}
			if c == "" {
				continue
			}
			if merged[i] == "" {
				merged[i] = c
			} else {
				merged[i] = merged[i] + " " + c
			}
		}
	}
	return merged
}

func splitTableCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "│")
	trimmed = strings.TrimSuffix(trimmed, "│")
	parts := strings.Split(trimmed, "│")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

func renderTableRow(cells []string) string {
	escaped := make([]string, len(cells))
	for i, c := range cells {
		escaped[i] = strings.ReplaceAll(c, "|", "\\|")
	}
	return "| " + strings.Join(escaped, " | ") + " |"
}

func renderTableSeparator(colCount int) string {
	cells := make([]string, colCount)
	for i := range cells {
		cells[i] = "---"
	}
	return "| " + strings.Join(cells, " | ") + " |"
}
