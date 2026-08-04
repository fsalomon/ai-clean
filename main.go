package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/atotto/clipboard"

	"github.com/TheAndruu/ai-clean/internal/clean"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ai-clean", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		stdinFlag   = fs.Bool("stdin", false, "read text from stdin and write cleaned text to stdout instead of using the clipboard")
		dryRun      = fs.Bool("dry-run", false, "print the cleaned text to stdout instead of writing it back to the clipboard")
		noRejoin    = fs.Bool("no-rejoin", false, "disable the wrapped-line rejoin heuristic (safer for pure code)")
		stripANSI   = fs.Bool("strip-ansi", false, "also strip ANSI / OSC escape sequences (off by default)")
		explain     = fs.Bool("explain", false, "print a one-line-per-stage summary to stderr describing what changed")
		showVersion = fs.Bool("version", false, "print version and exit")
	)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: %s [flags]\n\n", "ai-clean")
		fmt.Fprintln(stderr, "Cleans AI-CLI terminal output on the clipboard: strips border")
		fmt.Fprintln(stderr, "characters, trailing whitespace, and rejoins terminal-wrapped lines.")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "Flags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	opts := clean.Opts{
		StripANSI: *stripANSI,
		NoRejoin:  *noRejoin,
	}

	if *stdinFlag && *dryRun {
		fmt.Fprintln(stderr, "ai-clean: --dry-run applies to clipboard mode only; cannot combine with --stdin")
		return 2
	}

	if *stdinFlag {
		return runStdin(opts, *explain, stdin, stdout, stderr)
	}

	return runClipboard(opts, *dryRun, *explain, stdout, stderr)
}

func runStdin(opts clean.Opts, explain bool, stdin io.Reader, stdout, stderr io.Writer) int {
	in, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "ai-clean: read stdin: %v\n", err)
		return 1
	}
	out, stats := clean.Clean(string(in), opts)
	if _, err := io.WriteString(stdout, out); err != nil {
		fmt.Fprintf(stderr, "ai-clean: write stdout: %v\n", err)
		return 1
	}
	if explain {
		writeExplain(stderr, stats)
	}
	return 0
}

func runClipboard(opts clean.Opts, dryRun, explain bool, stdout, stderr io.Writer) int {
	if clipboard.Unsupported {
		fmt.Fprintln(stderr, clipboardHelpMessage())
		return 1
	}

	in, err := clipboard.ReadAll()
	if err != nil {
		fmt.Fprintf(stderr, "ai-clean: read clipboard: %v\n", err)
		if isLinuxClipboardMissing(err) {
			fmt.Fprintln(stderr, "")
			fmt.Fprintln(stderr, clipboardHelpMessage())
		}
		return 1
	}

	out, stats := clean.Clean(in, opts)

	if dryRun {
		fmt.Fprint(stdout, out)
		if explain {
			writeExplain(stderr, stats)
		}
		return 0
	}

	if err := clipboard.WriteAll(out); err != nil {
		fmt.Fprintf(stderr, "ai-clean: write clipboard: %v\n", err)
		return 1
	}

	lineCount := 0
	if out != "" {
		lineCount = strings.Count(out, "\n")
		if !strings.HasSuffix(out, "\n") {
			lineCount++
		}
	}
	fmt.Fprintf(stdout, "✓ cleaned %d line(s)\n", lineCount)
	if explain {
		writeExplain(stderr, stats)
	}
	return 0
}

func writeExplain(w io.Writer, s clean.Stats) {
	var b strings.Builder
	b.WriteString("ai-clean:\n")
	wrote := false
	if s.LeadingBorderLines > 0 {
		fmt.Fprintf(&b, "  leading border %q stripped from %d line(s)\n", s.LeadingBorderChar, s.LeadingBorderLines)
		wrote = true
	}
	if s.TrailingBorderLines > 0 {
		fmt.Fprintf(&b, "  trailing border %q stripped from %d line(s)\n", s.TrailingBorderChar, s.TrailingBorderLines)
		wrote = true
	}
	if s.DedentColumns > 0 {
		fmt.Fprintf(&b, "  dedented %d column(s)\n", s.DedentColumns)
		wrote = true
	}
	if s.BoxBorderLinesRemoved > 0 {
		fmt.Fprintf(&b, "  removed %d box-border line(s)\n", s.BoxBorderLinesRemoved)
		wrote = true
	}
	if s.BoxTablesRebuilt > 0 {
		fmt.Fprintf(&b, "  rebuilt %d box-drawing table(s) as markdown\n", s.BoxTablesRebuilt)
		wrote = true
	}
	if s.RejoinedLines > 0 {
		fmt.Fprintf(&b, "  rejoined %d wrapped line(s)\n", s.RejoinedLines)
		wrote = true
	}
	if s.BlankRunsCollapsed > 0 {
		fmt.Fprintf(&b, "  collapsed %d blank-line run(s)\n", s.BlankRunsCollapsed)
		wrote = true
	}
	if s.LeadingCapHit {
		b.WriteString("  ⚠ leading borders nested deeper than 3 layers — unusual input\n")
		wrote = true
	}
	if s.TrailingCapHit {
		b.WriteString("  ⚠ trailing borders nested deeper than 3 layers — unusual input\n")
		wrote = true
	}
	if s.UnclosedFence {
		b.WriteString("  ⚠ unclosed code fence detected; rejoin suppressed to EOF\n")
		wrote = true
	}
	if s.MarkdownTableSkipped > 0 {
		fmt.Fprintf(&b, "  ⚠ skipped %d markdown table guard(s) (left '|' borders intact)\n", s.MarkdownTableSkipped)
		wrote = true
	}
	if !wrote {
		b.WriteString("  no changes — input was already clean\n")
	}
	io.WriteString(w, b.String())
}

func isLinuxClipboardMissing(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no clipboard utilities") ||
		strings.Contains(msg, "executable file not found") ||
		strings.Contains(msg, "xclip") ||
		strings.Contains(msg, "xsel")
}

func clipboardHelpMessage() string {
	return "ai-clean needs a clipboard helper.\n" +
		"  Linux:   install one of: xclip, xsel, or wl-clipboard\n" +
		"           e.g. `sudo apt install xclip` or `sudo pacman -S xclip`\n" +
		"  macOS:   pbcopy/pbpaste ship with the OS — no install needed\n" +
		"  Windows: clip.exe ships with the OS — no install needed\n" +
		"\n" +
		"You can also pipe text in via --stdin to bypass the clipboard."
}
