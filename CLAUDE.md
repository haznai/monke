# monkeytype-tui

Terminal typing test inspired by MonkeyType, built in Go with Bubbletea.

## Philosophy

The core bet: **you don't need to type accurately if an LLM is fixing your output.** Type as fast as physically possible, let the LLM handle corrections. Your real speed is "corrected WPM", the throughput of correct characters regardless of typos. Traditional WPM penalizes errors. Corrected WPM ignores them, because the LLM will fix them anyway.

The gap between WPM and corrected WPM is what the LLM buys you. Over time, you want that gap to shrink to zero (meaning you're fast AND accurate), but in the meantime, the LLM makes your effective speed much higher than your raw accuracy would suggest.

This tool exists to train that workflow: type fast, build muscle memory for common patterns, and measure the metrics that actually matter when an LLM is your spellchecker.

## Corrected WPM

`corrected_wpm = (total_target_chars / 5) / time_minutes`

This counts every target character you attempted (even if you mistyped it) divided by the standard 5-char word. It's your effective throughput assuming perfect LLM correction. This is THE metric.

## Modes

- **words**: type N random words from a word list. Classic MonkeyType.
- **time**: type as many words as possible in N seconds.
- **quote**: type a random English quote (short/medium/long/thicc).
- **ngram**: drill common character sequences (bigrams/trigrams). Lesson-by-lesson progression, must hit 120 WPM to advance. Builds muscle memory for the most frequent English patterns.

## Quick Reference

```
go run .              # launch the TUI
go run . fetch        # re-fetch datasets from MonkeyType GitHub
go test ./... -v      # run all tests
go build -o monkeytype-tui .
```

Use `go` (whatever is on PATH), not a hardcoded binary path.

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
  dataset/                    # word list, quote, and ngram management
    dataset.go                #   JSON parsing, random word/quote selection
    ngrams.go                 #   hardcoded bigram/trigram data, lesson generation
    fetch.go                  #   HTTP fetch from MonkeyType GitHub, local caching
    dataset_test.go           #   20 tests (uses testdata/ fixtures + httptest)
    testdata/                 #   test fixture JSON files
  history/                    # test result persistence
    history.go                #   JSON file store, personal bests, averages
    history_test.go           #   16 tests (uses t.TempDir(), real file I/O)
  menu/                       # mode selection screen
    menu.go                   #   mode/value/wordlist picker with ngram support
  theme/                      # Lipgloss styles (MonkeyType dark theme)
    theme.go
  llm/                        # LLM spellcheck integration
    llm.go                    #   post-test spellcheck via LLM API
```

## Ngram Mode

Inspired by [ngram-type](https://github.com/ranelpadon/ngram-type). Drills the most common English character sequences to build muscle memory.

- 200 bigrams ("th", "he", "in", ...) and 200 trigrams ("the", "and", "ing", ...) ranked by frequency, hardcoded in `dataset/ngrams.go`
- Scope: top 50 / 100 / 150 / 200 from the frequency list
- Lesson generation: shuffle the scoped pool, pair into chunks of 2, repeat each chunk 3 times (e.g. "th he th he th he")
- Progression: must hit 120 WPM on a lesson to advance. Fail = retype same lesson. Pass = next lesson.
- Lesson counter shown in header ("lesson 5/25"), last attempt WPM shown in footer

## Typing Mechanics

- Space always advances to next word (even if wrong)
- Backspace deletes last char in current word
- Ctrl+W / Alt+Backspace deletes entire current word
- Tab restarts the test (or current ngram lesson)
- Esc returns to menu
- Test passes only if every word is correct

## Datasets

Cached at `~/.monkeytype-tui/datasets/`. Fetched from MonkeyType GitHub on first run.
Word lists: english (200), english_1k, english_5k, english_10k.
Quotes: 6400+ English quotes in 4 length categories.
Ngrams: hardcoded, no fetch needed.

## History

Stored at `~/.monkeytype-tui/history.json`. Every test result is saved.
Tracks personal bests per mode/wordlist/duration combo.

## Testing

This codebase must be well tested. Every behavioral path needs a test. If a bug is found, a regression test goes in before the fix. New features ship with tests or they don't ship.

- NO MOCKS. Unit tests use real data, real files (t.TempDir()), real calculations.
- Exception: httptest.NewServer for HTTP fetch tests.
- Test fixtures live in `testdata/` directories.
- Run only touched package tests during dev: `go test ./internal/typing/ -v`
- Integration tests in `internal/app/` exercise the full pipeline (engine -> stats -> history -> results).
- Write tests with hand-verified math first, then implement. Red/green TDD.
- Tests must test the actual thing. If a test passes with the implementation deleted, it's worthless.
- Cover edge cases aggressively: empty input, single word, punctuation in quotes, LLM mangling correctly-typed words, mismatched word counts.
- When fixing a bug, write the test that would have caught it FIRST.

## Adding Features

- New test modes: add to `internal/menu/menu.go`, handle in `app.go:startTypingTest()`
- New stats: add to `stats.go:Calculate()`, write tests with hand-verified numbers first
- New key bindings: handle in `typing.go:handleKey()`
- Themes: add to `internal/theme/theme.go`
- Ngram data: edit `dataset/ngrams.go` (hardcoded slices, no fetch)
