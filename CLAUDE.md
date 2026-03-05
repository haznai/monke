# monkeytype-tui

Terminal typing test inspired by MonkeyType, built in Go with Bubbletea.
Optimized for the "type fast, LLM fixes your typos" workflow.

## Quick Reference

```
go run .              # launch the TUI
go run . fetch        # re-fetch datasets from MonkeyType GitHub
go test ./... -v      # run all tests (86 tests)
go build -o monkeytype-tui .
```

Go binary location: `/opt/homebrew/bin/go`

## Architecture

```
main.go                       # entry point, fetch subcommand
internal/
  app/                        # top-level Bubbletea model, screen routing
    app.go                    #   loading -> menu -> typing -> results flow
    typing.go                 #   typing test screen (keyboard handling, word rendering)
    results.go                #   results screen (WPM, accuracy, corrected WPM display)
    integration_test.go       #   end-to-end tests: engine -> stats -> history
  typing/                     # core state machine (NO TUI dependency)
    engine.go                 #   word tracking, input buffer, correctness, timing
    engine_test.go            #   29 tests covering every behavior
  stats/                      # pure calculation (NO I/O, NO state)
    stats.go                  #   WPM, raw WPM, corrected WPM, accuracy, consistency
    stats_test.go             #   14 tests with hand-verified math
  dataset/                    # word list and quote management
    dataset.go                #   JSON parsing, random word/quote selection
    fetch.go                  #   HTTP fetch from MonkeyType GitHub, local caching
    dataset_test.go           #   20 tests (uses testdata/ fixtures + httptest)
    testdata/                 #   test fixture JSON files
  history/                    # test result persistence
    history.go                #   JSON file store, personal bests, averages
    history_test.go           #   16 tests (uses t.TempDir(), real file I/O)
  menu/                       # mode selection screen
    menu.go                   #   words/time/quote, word list, count picker
  theme/                      # Lipgloss styles (MonkeyType dark theme)
    theme.go
```

## Key Concepts

### Corrected WPM
The big idea: `corrected_wpm = (total_target_chars / 5) / time_minutes`.
This is your effective WPM assuming an LLM fixes all your typos.
The gap between WPM and corrected WPM is what the LLM "buys" you.
Over time, you want this gap to shrink to zero.

### Typing Mechanics
- Space always advances to next word (even if wrong)
- Backspace deletes last char in current word
- Ctrl+W / Alt+Backspace deletes entire current word (Cmd+Backspace on macOS)
- Tab restarts the test
- Esc returns to menu
- Test passes only if every word is correct

### Datasets
Cached at `~/.monkeytype-tui/datasets/`. Fetched from MonkeyType GitHub on first run.
Word lists: english (200), english_1k, english_5k, english_10k.
Quotes: 6400+ English quotes in 4 length categories.

### History
Stored at `~/.monkeytype-tui/history.json`. Every test result is saved.
Tracks personal bests per mode/wordlist/duration combo.

## Testing Rules

- NO MOCKS. Unit tests use real data, real files (t.TempDir()), real calculations.
- Exception: httptest.NewServer for HTTP fetch tests.
- Test fixtures live in `testdata/` directories.
- Run only touched package tests during dev: `go test ./internal/typing/ -v`
- Integration tests in `internal/app/` exercise the full pipeline.

## Adding Features

- New test modes: add to `internal/menu/menu.go`, handle in `app.go:startTypingTest()`
- New stats: add to `stats.go:Calculate()`, write tests with hand-verified numbers first
- New key bindings: handle in `typing.go:handleKey()`
- Themes: add to `internal/theme/theme.go`
- LLM spellcheck: will be wired into the results flow after test completion (API TBD)
