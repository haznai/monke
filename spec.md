# monkeytype-tui

A terminal typing test inspired by MonkeyType, built in Go with Bubbletea.
The twist: optimized for the "type fast, LLM fixes your typos" workflow.

## Why

Modern writing workflow: blast through text at max speed, let an LLM spellcheck after.
This trains you to type fast without the mental overhead of perfect accuracy,
while the "all words must be correct" completion gate pushes you toward accuracy over time.

The gap between raw WPM and corrected WPM is your improvement metric.

## Core Mechanics

### Typing Flow

1. Words appear in a horizontal line (wrapping). Current word is highlighted.
2. Type characters, they appear colored: correct = dim/green, incorrect = red.
3. **Space** always advances to the next word, even if current word is wrong.
   Incorrect words stay marked red. Cursor moves to next word.
4. **Backspace** deletes the last character in the current word.
5. **Cmd+Backspace** nukes the entire current word (works natively in macOS terminals).
   Ctrl+W as fallback for non-macOS environments.
6. **Tab** restarts the current test from scratch.
7. At end of test: if all words are correct, test is "passed". Otherwise "failed"
   (but stats are still recorded either way).

### What "Correct" Means

A word is correct if and only if the typed string exactly matches the target word.
Skipped words (spaced past without typing) count as incorrect.
Extra characters count as incorrect.

## Test Modes

### Word Mode
- Fixed number of words: 10, 25, 50, 100
- Pulled randomly from the selected word list
- Test ends when last word is submitted (space or enter)

### Time Mode
- Fixed duration: 15s, 30s, 60s, 120s
- Words generate continuously (buffer ahead of cursor)
- Test ends when timer expires

### Quote Mode
- Full quotes from MonkeyType's quote dataset
- Filterable by length: short (<100 chars), medium (100-300), long (300-600), thicc (600+)
- Test ends when last word is submitted

## Datasets

Fetched from MonkeyType's GitHub repo (`monkeytypegame/monkeytype`):

### Word Lists (JSON)
- `english.json` (200 common words)
- `english_1k.json` (1,000)
- `english_5k.json` (5,000)
- `english_10k.json` (10,000)

Format:
```json
{
  "name": "english_1k",
  "orderedByFrequency": true,
  "words": ["the", "of", "to", "and", ...]
}
```

### Quotes (JSON)
- `english.json` from `frontend/static/quotes/`

Format:
```json
{
  "language": "english",
  "groups": [[0, 100], [101, 300], [301, 600], [601, 9999]],
  "quotes": [
    { "text": "...", "source": "Author Name", "id": 1, "length": 45 }
  ]
}
```

### Storage
- Datasets cached locally in `~/.monkeytype-tui/datasets/`
- Fetched on first run or via `monkeytype-tui fetch` command
- Raw JSON files, loaded into memory at startup

## Stats & Metrics

### Per-Test Metrics

| Metric | Formula |
|--------|---------|
| **WPM** | `(correct_chars / 5) / time_minutes` |
| **Raw WPM** | `(all_typed_chars / 5) / time_minutes` |
| **Corrected WPM** | `(total_target_chars / 5) / time_minutes` (assumes LLM fixes all typos, so your effective output is the full text) |
| **Accuracy** | `correct_chars / total_typed_chars * 100` |
| **Consistency** | `100 - coefficient_of_variation(per_second_wpm)` |
| **Correction Delta** | `corrected_wpm - wpm` (the WPM you "gain" from LLM spellcheck) |

### The Key Insight: Corrected WPM

Standard WPM only counts correct characters. But if you're using an LLM spellcheck:
- You typed the whole text in X seconds
- The LLM fixes your typos
- Your *effective* output is the full correct text
- Corrected WPM = what your WPM *would be* if every word you typed was correct

This is the number that matters for the "type fast + LLM cleanup" workflow.

### Results Screen

```
  wpm          142      raw wpm      168
  corrected    161      accuracy     84.5%
  consistency  89%      delta        +19 wpm

  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  WPM over time graph (sparkline or bar)
  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  correct 42/50 words | test: FAILED
  [tab] restart  [enter] next test  [esc] menu
```

### LLM Spellcheck (Future)

- After test completion, send typed text to an LLM API for correction
- Compare corrected output to target text
- Calculate actual corrected accuracy (not assumed 100%)
- API details TBD (user will provide later)
- For now: assume LLM corrects all typos perfectly (optimistic corrected WPM)

## Stats Persistence

### History File
- Stored at `~/.monkeytype-tui/history.json`
- Append-only log of every test result

```json
{
  "tests": [
    {
      "timestamp": "2026-03-05T14:30:00Z",
      "mode": "words",
      "mode_value": 50,
      "word_list": "english_1k",
      "wpm": 142,
      "raw_wpm": 168,
      "corrected_wpm": 161,
      "accuracy": 84.5,
      "consistency": 89,
      "correct_words": 42,
      "total_words": 50,
      "passed": false,
      "time_seconds": 28.4,
      "typed_text": "the quick brown fox ...",
      "target_text": "the quick brown fox ..."
    }
  ]
}
```

### Personal Bests
- Track PBs per mode/word-list/duration combo
- Show PB indicator on results screen when beaten

## UI / Look & Feel

### Layout

```
  ╭─────────────────────────────────────────────╮
  │  monkeytype-tui                   words 50  │
  │                                  english_1k  │
  ├─────────────────────────────────────────────┤
  │                                             │
  │  the quick brown fox jumps over the lazy    │
  │  dog while ███████ something about typing   │
  │  fast and not caring about mistakes too     │
  │  much because an llm will fix it later      │
  │                                             │
  ├─────────────────────────────────────────────┤
  │  wpm: 142  |  raw: 168  |  acc: 84%        │
  ╰─────────────────────────────────────────────╯
```

- Current word: highlighted/underlined
- Typed correct chars: dim (fade into background, like MonkeyType)
- Typed incorrect chars: red/bold
- Untyped words: muted gray
- Skipped (incorrect) words: red underline, stay visible
- Smooth cursor/caret on current character position
- Live WPM counter in footer (updates every second)

### Color Themes
- Start with one dark theme (MonkeyType's default vibe)
- Theme system for later expansion

### Navigation
- Mode selection screen on launch (words/time/quote, word list, count/duration)
- Arrow keys + Enter to select
- Esc to go back
- Minimal chrome, maximum typing area

## Tech Stack

- **Language**: Go
- **TUI Framework**: Bubbletea (Elm architecture, tight event loop)
- **Styling**: Lipgloss
- **Config/Data**: JSON files in `~/.monkeytype-tui/`

## Project Structure

```
monkeytype-tui/
├── main.go
├── go.mod
├── go.sum
├── internal/
│   ├── app/          # top-level bubbletea model, routing between screens
│   ├── typing/       # typing test model, input handling, word tracking
│   ├── results/      # results screen model
│   ├── menu/         # mode selection screen
│   ├── stats/        # WPM/accuracy/consistency calculations
│   ├── dataset/      # word list and quote loading, fetching
│   ├── history/      # test history persistence
│   └── theme/        # colors, styles
├── datasets/         # gitignored, cached word lists
└── README.md
```

## Keyboard Reference

| Key | Action |
|-----|--------|
| `a-z`, `'`, `-`, etc. | Type character |
| `Space` | Submit current word, advance to next |
| `Backspace` | Delete last character in current word |
| `Cmd+Backspace` | Delete entire current word (Ctrl+W fallback) |
| `Tab` | Restart test |
| `Esc` | Back to menu / quit |
| `Enter` | (results screen) start next test |

Note: Cmd+Backspace works natively in macOS terminals.
Ctrl+W kept as fallback for Linux/other environments.

## MVP Scope

Phase 1 (build now):
- Word mode (10, 25, 50, 100) with english_1k
- Time mode (15s, 30s, 60s)
- Core typing mechanics (space to advance, backspace, ctrl+w, tab restart)
- WPM, raw WPM, corrected WPM, accuracy
- Results screen with all metrics
- Dataset fetching from MonkeyType GitHub
- Test history persistence

Phase 2 (later):
- Quote mode
- More word lists (200, 5k, 10k)
- LLM spellcheck API integration (actual correction, not assumed)
- Personal bests tracking
- WPM-over-time graph
- Themes
- Consistency metric
- Punctuation/numbers toggles
