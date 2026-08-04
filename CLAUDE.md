# CLAUDE.md

Project context for Claude Code sessions on this repo.

## Fork status

Fork of [TheAndruu/ai-clean](https://github.com/TheAndruu/ai-clean). Built from source at a pinned commit by the clipboard-manager integration that consumes it.

Carried here, not upstream (no PR opened):

- `borderChars` covers the whole block-element run, so Claude Code's `▎` quote bars get stripped. Upstream lists only `▌`, a visually identical but different rune, so quoted blocks came through untouched.
- `table.go` — box-drawing tables rebuilt as Markdown. This previously ran as a separate binary piping into `ai-clean`; it belongs inside the pipeline, because the guard that protects Markdown tables runs too late to help a box-drawing render.

The module path stays `github.com/TheAndruu/ai-clean` on purpose, so upstream merges don't conflict on every import. Sync with `git fetch upstream && git merge upstream/master`.

## What this is

`ai-clean` — a small Go CLI that reads text from the clipboard, normalizes it (strips terminal chrome, trims whitespace, optionally rejoins wrapped lines), and writes it back. Motivating use case: cleaning up output copied from AI CLI tools (Claude Code, Copilot CLI) that arrives with border characters like `│`, padding, and terminal-hard-wrapped lines.

## Architecture

- `main` (`./main.go`) — flag parsing, clipboard I/O via `github.com/atotto/clipboard`. Work is split into `run(args, stdin, stdout, stderr) int` so `main_test.go` exercises the CLI without spawning a subprocess.
- `internal/clean` — the cleanup pipeline. Public surface: `Clean(text string, opts Opts) (string, Stats)`. `Stats` reports per-stage counts and warning flags; `main.go` consumes it for `--explain`.
- `cmd/wasm` — build-tagged `js && wasm` entry point that exposes `Clean` to JavaScript via `syscall/js`. Reuses `internal/clean` unchanged; powers the in-browser demo at https://ai-clean.dev.

## Cleanup pipeline (order matters)

Steps 3–6 run inside a fix-point loop in `clean.go` until a full pass makes no change. Each stage can produce input another would clean further (a trailing-strip can turn a mixed line into pure box chrome; rejoin can expose leading whitespace from the merged tail). Convergence is guaranteed because every changing pass strictly shrinks the document.

1. `ansi.go` — optional ANSI/OSC strip. Off by default; only runs if `Opts.StripANSI` is true. Once at input, before the loop.
2. `table.go` — `rebuildBoxTables`. Converts a `┌─┬─┐` box-drawing table back into Markdown table syntax, merging cells the terminal wrapped across several physical lines. Once at input, before the loop, and deliberately so: the stage consumes the rule lines that define row boundaries, so running it per-pass would let cell content that happens to look like a rule line be re-parsed, breaking idempotency. Running it once is safe because by the end of the loop `stripFullBoxBorders` has deleted every rule line, leaving a second `Clean()` nothing to rebuild. Its output is `|`-shaped, which the markdown-table guard in step 4 then protects.
3. `box.go` — `stripFullBoxBorders`. Removes lines that are pure box-drawing chrome (every non-WS rune in `U+2500..U+257F` AND at least one horizontal rule: `─ ━ ═ ╌ ╍`). Catches `┌─┐`/`└─┘`/`═══` framing without touching markdown rules (`---`, `***`, `===`).
4. `leading.go` — `stripLeadingChrome`. Loops over: dedent the minimum-common leading whitespace, then strip a uniform leading border char (`│ ┃ | > ┆ ╎ ┊ ┇`, plus the block-element quote bars `▉▊▋▌▍▎▏`) when it appears on ≥80% of non-empty lines. **Markdown-table guard**: if the candidate is `|` and ≥50% of border-having lines also have an interior `|`, the strip is skipped.
5. `trailing.go` — `stripTrailingChrome`. Mirror for the right side. Border-char strip must run before whitespace trim — that's why both are bundled here.
6. `rejoin.go` — `rejoinWrapped`. Conservative wrapped-line merge. Skipped when `Opts.NoRejoin` is true. Guards: fenced-code detection (sets `Stats.UnclosedFence`), leading whitespace on either side, list/heading markers, table rows, sentence terminators, minimum doc-width floor (`rejoinMinDocWidth = 40`).
7. `clean.go` — `collapseBlankRuns`. Runs of 3+ blank lines collapse to 2. Once after the loop.

CRLF/CR normalization happens at input, before the loop: `\r\n` → `\n`, then any remaining `\r` → `\n` (handles old-Mac endings and stray CRs that would break idempotency).

## Conventions

- **Pure Go, no Cgo.** Keeps cross-compilation trivial. The clipboard library was chosen specifically to avoid Cgo. Don't introduce Cgo deps.
- **Tests are the correctness gate.** `internal/clean/clean_test.go` is table-driven plus a `testdata/`-driven full-pipeline test. New pipeline behavior should land with a test case. When fixing a bug, add the regression case to the table first.
- **Idempotency is an invariant.** `Clean(Clean(x), opts) == Clean(x, opts)` must hold. Enforced by `TestCleanIdempotentOverTestdata` and `FuzzClean`. If you touch the pipeline, run the fuzzer for ≥30s — it has caught real bugs.
- **Pipeline ordering is fragile.** Don't move work outside the fix-point loop unless you're sure it can't introduce new chrome the other stages would catch.
- **`borderChars` covers the whole block-element run** `U+2589..U+258F` (`▉▊▋▌▍▎▏`), not just `▌`. CLIs pick arbitrary widths out of it for quote bars and the glyphs are indistinguishable at terminal font sizes — Claude Code uses `▎` (U+258E). `U+2588` (`█`) is excluded on purpose: a full block is usually graphics fill or a progress bar, not chrome.
- **The 80% border-detection threshold** (`borderThreshold` in `leading.go`) is intentional. Don't raise to 100% — that breaks on real-world output with one occasional missing border.
- **The 50% markdown-table threshold** (`markdownTableThreshold`) is lower than the border threshold because separators (`|---|---|`) and continuation rows dilute the interior-`|` count.
- **The 3-pass `nestingWarnThreshold` is a warning, not a cap.** Loops run until convergence; this only sets `Stats.LeadingCapHit` / `TrailingCapHit`. The hard cap (`pipelineSafetyCap = 100`) makes a heuristic bug fail loud instead of looping forever.
- **Unicode care.** Border characters are multi-byte runes (`│` is 3 bytes). Use `[]rune` for indexing into lines, not `[]byte`.
- **No comments explaining what code does.** Only the *why*. Test names document behavior.

## Build / test

```sh
go test ./...                                                    # all tests
go test ./internal/clean -fuzz=FuzzClean -fuzztime=30s           # idempotency fuzz
go test ./internal/clean -bench=BenchmarkClean -benchmem -run=^$ # baseline perf
go build -o ai-clean .                                           # local CLI binary
GOOS=js GOARCH=wasm go build -o web/ai-clean.wasm ./cmd/wasm     # WASM for the web demo
```

## Adding example test cases

Fixtures live in `testdata/examples/` as `<characteristic>_sample.txt` / `<characteristic>_expected.txt` pairs. Naming describes the behavior exercised, not the source tool. Workflow: save the raw input as `_sample.txt`, run `go test ./internal/clean -run TestCleanFromTestdata -update` to generate the expected file, review with `git diff`. If the expected file looks wrong, the case revealed a rule gap — edit `internal/clean/*.go`, re-run with `-update`. Without `-update`, missing/mismatched expected files fail the test.

## Web demo (cmd/wasm + web/)

Live at https://ai-clean.dev (custom domain, IONOS DNS) and `theandruu.github.io/ai-clean/`.

- `web/index.html`, `web/style.css`, `web/script.js` — static page with light/dark `prefers-color-scheme`, debounced live preview, copy button, stats panel mirroring `--explain`.
- `web/CNAME` — `ai-clean.dev`. Required for custom domain when Pages source is "GitHub Actions" (it's not auto-committed in that mode).
- `.github/workflows/pages.yml` — builds WASM, copies `wasm_exec.js` from `$(go env GOROOT)/lib/wasm/`, deploys to Pages on every push to master.
- `web/ai-clean.wasm` and `web/wasm_exec.js` are gitignored; CI generates them from the current Go toolchain so they always match.

## Releases

Distribution via GoReleaser (`.goreleaser.yml`) and `.github/workflows/release.yml` triggered on `v*` tags. Cut a release: `git tag v0.X.0 && git push --tags`. CI builds darwin/linux/windows × amd64/arm64 (windows/arm64 skipped) and pushes a Homebrew cask to `TheAndruu/homebrew-tap`. The cask's `hooks.post.install` clears `com.apple.quarantine` so the unsigned binary doesn't trip Gatekeeper. Casks are macOS-only — Linux users go through the `curl` one-liner. Shell completions ship in every release archive and the cask installs them.
