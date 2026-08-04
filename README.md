# AI Clean

[![Release](https://img.shields.io/github/v/release/TheAndruu/ai-clean)](https://github.com/TheAndruu/ai-clean/releases/latest)
[![Build](https://github.com/TheAndruu/ai-clean/actions/workflows/test.yml/badge.svg)](https://github.com/TheAndruu/ai-clean/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/TheAndruu/ai-clean.svg)](https://pkg.go.dev/github.com/TheAndruu/ai-clean)
[![Go Report Card](https://goreportcard.com/badge/github.com/TheAndruu/ai-clean)](https://goreportcard.com/report/github.com/TheAndruu/ai-clean)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
![demo](demo/demo.gif)

Clean up output you copied from AI CLI tools (Claude Code, GitHub Copilot CLI, and similar) so it pastes into chat, GitHub issues, docs, or another terminal without the usual mess: trailing whitespace padding, terminal-wrapped lines, and left/right border characters like `│` or `|`.

Workflow: **select → copy → `ai-clean` → paste**.

**[Try it in your browser →](https://ai-clean.dev)** — paste messy text and see it cleaned, no install required. The page runs the same Go cleanup pipeline as the CLI, compiled to WebAssembly; nothing is uploaded.

## What it fixes

Before (copied straight from a bordered CLI):

```
  │ Here is a small example showing the structure that copilot's CLI │
  │ produces. Each line has a left and right border character.       │
  │                                                                  │
  │     def greet(name):                                             │
  │         return f"hello, {name}"                                  │
```

After `ai-clean`:

```
Here is a small example showing the structure that copilot's CLI produces. Each line has a left and right border character.

    def greet(name):
        return f"hello, {name}"
```

Code-block indentation is preserved; prose is rejoined; borders and trailing whitespace are gone.

## Install

### macOS — Homebrew (recommended)

```sh
brew install TheAndruu/tap/ai-clean
```
Installs shell completions for bash, zsh, and fish automatically. Linux users: use the one-line installer below — Homebrew casks are macOS-only.

To upgrade later:

```sh
brew update
brew upgrade ai-clean
```

### One-line installers (macOS / Linux / Windows)

Copy and paste the command for your platform.

**macOS — Apple Silicon (M1 / M2 / M3 / M4)**

```sh
curl -fsSL https://github.com/TheAndruu/ai-clean/releases/latest/download/ai-clean_darwin_arm64.tar.gz | sudo tar -xz -C /usr/local/bin ai-clean
```

**macOS — Intel**

```sh
curl -fsSL https://github.com/TheAndruu/ai-clean/releases/latest/download/ai-clean_darwin_amd64.tar.gz | sudo tar -xz -C /usr/local/bin ai-clean
```

**Linux — x86_64**

```sh
curl -fsSL https://github.com/TheAndruu/ai-clean/releases/latest/download/ai-clean_linux_amd64.tar.gz | sudo tar -xz -C /usr/local/bin ai-clean
```

**Linux — arm64**

```sh
curl -fsSL https://github.com/TheAndruu/ai-clean/releases/latest/download/ai-clean_linux_arm64.tar.gz | sudo tar -xz -C /usr/local/bin ai-clean
```

Each `curl` command above extracts only the `ai-clean` binary into `/usr/local/bin`. Verify with `ai-clean --version`. To upgrade, re-run the same command.

**Windows — PowerShell**

```powershell
$dest = "$env:LOCALAPPDATA\Programs\ai-clean"
$tmp  = "$env:TEMP\ai-clean.zip"
Invoke-WebRequest -Uri https://github.com/TheAndruu/ai-clean/releases/latest/download/ai-clean_windows_amd64.zip -OutFile $tmp
New-Item -ItemType Directory -Force -Path $dest | Out-Null
Expand-Archive -Force $tmp -DestinationPath $dest
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$dest", "User")
```

Open a new PowerShell window for the updated `PATH` to take effect, then verify with `ai-clean --version`. To upgrade, re-run the same command.

### From source

```sh
go install github.com/TheAndruu/ai-clean@latest
```

### Linux requirement

`ai-clean` shells out to a system clipboard helper on Linux. Install one:

```sh
sudo apt install xclip          # Debian / Ubuntu
sudo pacman -S xclip            # Arch
sudo dnf install xclip          # Fedora
# Wayland: sudo apt install wl-clipboard
```

## Usage

```
ai-clean              # read clipboard, clean, write back
ai-clean --dry-run    # print cleaned text instead of writing back to clipboard
ai-clean --stdin      # read stdin, write stdout (composable in pipelines)
ai-clean --no-rejoin  # skip the wrapped-line rejoin (safer when pasting pure code)
ai-clean --strip-ansi # also strip ANSI / OSC escape sequences
ai-clean --explain    # print a per-stage summary to stderr (what was stripped and why)
ai-clean --version
```

Examples:

```sh
# Most common: copy from terminal, run, paste into your editor.
ai-clean

# Pipe a captured log file through it.
cat session.log | ai-clean --stdin > clean.txt

# Verify what would happen without modifying the clipboard.
ai-clean --dry-run

# See exactly what the cleanup did (useful for debugging unexpected output).
ai-clean --explain
```

`--explain` writes a short summary to stderr after the cleaned text — for example:

```
ai-clean:
  leading border '│' stripped from 23 line(s)
  trailing border '│' stripped from 22 line(s)
  removed 2 box-border line(s)
  rejoined 8 wrapped line(s)
  ⚠ skipped 1 markdown table guard(s) (left '|' borders intact)
```

Lines for stages that did nothing are suppressed; warnings (`⚠`) only appear when the relevant condition was hit.

## How it works

The cleanup runs in a fixed order, designed to be safe across plain prose, plain code, and mixed content:

1. **Optional ANSI / OSC strip** (only with `--strip-ansi`). Off by default because terminals usually strip these on copy already.
2. **Rebuild box-drawing tables.** A Markdown table the CLI rendered as a `┌─┬─┐` box is converted back into Markdown table syntax, with cells the terminal wrapped over several lines merged back into one. Runs once, on the raw input, before any stage strips the rule lines that define the row boundaries; text around the table is untouched and flows on into the steps below.
3. **Strip full-box borders.** Removes lines that are pure box-drawing chrome (top/bottom horizontal rules with corners, like `┌─────┐` and `└─────┘`, or double-line rules like `═══════`). Markdown ASCII rules (`---`, `***`, `===`) are treated as content and left alone.
4. **Strip leading chrome.** Computes the minimum leading-whitespace count across non-empty lines and dedents — preserving relative indentation of code blocks. Then detects a uniform border character (`│`, `|`, `>`, the block-element quote bars `▌`/`▎`, etc.) appearing on ≥80% of non-empty lines and strips it. **Markdown tables are preserved**: if the candidate border is `|` and the rows look like a table (interior `|` characters present), the strip is skipped.
5. **Strip trailing chrome.** Mirror of step 4 for the right side: detects a uniform trailing border character (looking past trailing whitespace), strips it, then trims trailing whitespace per line. Same markdown-table guard.
6. **Rejoin wrapped lines.** Conservative heuristic that merges adjacent prose lines only when all of the following hold: the previous line doesn't end in sentence-terminating punctuation, the next doesn't start with a capital / list marker / heading marker, neither side is a markdown table row, neither side has leading whitespace, and the document's longest line is at least 40 chars (a proxy for terminal-wrapped output). Skipped entirely inside fenced code blocks. Use `--no-rejoin` to disable.
7. **Cosmetic blank-line collapse.** Runs of 3+ consecutive blank lines collapse to 2.

Steps 3–6 run inside a fix-point loop — each stage can produce input the others would clean further (a trailing-strip can turn a previously-mixed line into pure box chrome; rejoin can expose leading whitespace from the merged tail). The loop terminates as soon as a full pass makes no change, which is guaranteed because every changing pass strictly shrinks the document.

## License

MIT
