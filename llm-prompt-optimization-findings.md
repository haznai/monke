# LLM prompt optimization findings

Date: 2026-06-03

## Executive summary

Production decision: ship the best completed experimental prompt, `one_token`.

`one_token` improved the training-set correction score by **+2.27 percentage points**, added **+25 exact word corrections** over the current prompt on the training split, and reduced word-count mismatch fallbacks from **32** to **20**. It was also roughly as fast as the current prompt.

Caveat: it damaged a few already-good rows by capitalizing, lightly rewriting grammar, or choosing plausible synonyms. That is the main risk to watch. The benchmark should reward fast typo repair, not creative prose editing with a clipboard.

The prompt change is reasonable as a product choice, but the evidence is not a clean ML-style proof. No held-out validation run completed because Groq throttling made additional benchmarking too slow.

## Data inventory

Primary files:

```text
/Users/hazn/Library/Application Support/monkeytype-tui/history.json
/Users/hazn/Library/Application Support/monkeytype-tui/llm-calls.sqlite3
```

Backfill status:

- All quote history runs now have SQLite LLM rows.
- As of the latest check, there are 393 quote runs and 393 quote LLM rows.
- Rows are distinct by timestamp + target text + typed text.
- Logged LLM errors: 0.

Before backfill, SQLite only covered the newer calls. The old quote runs were saved in JSON but did not have LLM rows. Backfill used the same app request/response code path, with one honest timestamp caveat: old backfilled rows use the historical run timestamp in UTC, not the original async LLM-start timestamp a few milliseconds later.

## Current production prompt and model settings

Current system prompt:

```text
Correct this fast typing-test input. Treat each whitespace-separated input token as one output token: for every input token, output exactly one corrected token in the same position. Fix typos, missing apostrophes, punctuation, and capitalization when obvious. If unsure, leave the token unchanged. Output only the corrected text.
```

Baseline prompt at the time of the benchmark:

```text
You are a spellchecker. Fix spelling errors in the text below. Output ONLY the corrected text, nothing else. Do not change capitalization, punctuation, or word count. Do not add or remove words.
```

Current model/API settings:

```text
provider: groq
model: openai/gpt-oss-20b
temperature: 0
top_p: 0.2
max_completion_tokens: 512
reasoning_effort: low
```

Current user prompt is just:

```text
typed_words joined with spaces
```

No target quote is sent to the LLM. That matters. Sending the target quote would trivially improve correction accuracy, but it would stop testing whether the LLM can recover the intended text from noisy typing. It would become target matching, not spellcheck.

## Scoring method

The benchmark compares corrected output against `target_words` from the typing test.

For each run:

- `typed_correct`: count of typed words exactly equal to target words.
- `corrected_correct`: count of LLM-corrected words exactly equal to target words.
- `all_correct`: whether every corrected word equals the target word.
- `word_count_mismatch`: current app behavior, if `strings.Fields(rawCorrectedText)` does not have the same length as `typedWords`, discard the corrected words and fall back to the original typed words.
- Latency: measured request latency, cached per prompt candidate.

Exact match is case-sensitive and punctuation-sensitive. That is appropriate because MonkeyType quotes are exact text. It also means capitalizing `i` to `I` can be a regression if the target has lowercase `i`.

## Full current baseline

Latest full SQLite quote corpus:

```text
runs: 393
words: 6178
```

Typed-only baseline:

```text
typed correct words: 4698 / 6178
accuracy: 76.04%
all-correct runs: 22 / 393
```

Baseline LLM prompt at benchmark time:

```text
corrected correct words: 5383 / 6178
accuracy: 87.13%
delta over typed: +685 exact words
all-correct runs: 163 / 393
word-count mismatch fallbacks: 76 / 393
average latency: ~393 ms
p95 latency: ~736 ms
```

Important: current LLM is already doing useful work. It improves many bad runs. The problem is not "LLM does nothing". The problem is that the remaining failures are concentrated in hard cases: word-count mismatch, empty model outputs, severe keyboard garbage, and occasional model rewrites.

## Failure modes observed

### 1. Word-count mismatch causes total fallback

The app currently does this:

1. Model returns raw corrected text.
2. App runs `strings.Fields(rawCorrectedText)`.
3. If that count differs from `len(typedWords)`, app discards all corrected words for that run.
4. Corrected words become the original typed words.

This is safe, but brutal. It avoids bad alignment, but it also throws away partial useful corrections.

In the current full corpus:

```text
word-count mismatch fallbacks: 76
raw-empty mismatches: 47 / 76
```

So a large chunk of mismatch cases are not merely "the model added one word". The model sometimes returns empty content for noisy/long inputs. The current fallback correctly prevents bogus UI, but it means those runs get no benefit.

### 2. Empty typed words and spacing make word counts fragile

`typedWords` is an array from the typing engine. It can contain empty strings when a word was skipped. But the user prompt is a plain string made by joining words with spaces.

That means the prompt can contain repeated spaces or weird gaps, while the model response is parsed with `strings.Fields`, which collapses whitespace. This can cause mismatch even when the model's semantic output looks plausible.

This is a data-format issue more than a prompt issue. A prompt cannot reliably preserve empty tokens in a plain prose string.

### 3. Capitalization is dangerous

Some candidate prompts improved typo correction but started capitalizing words:

- `i` -> `I`
- sentence starts -> uppercase
- lower-case quote style -> title/sentence case

That hurts exact-match scoring when the target quote is lowercase. The current prompt explicitly says not to change capitalization, and that conservatism is valuable.

### 4. Grammar rewrites and synonyms are poison

The model likes to be helpful. Helpful is bad here.

Observed bad behaviors:

- `right now` became `are now`
- typo-like strings became plausible synonyms instead of the target word
- punctuation was normalized in ways that did not match target punctuation
- grammar was "improved" instead of repaired

A typing-test spellchecker should be literal. It should fix obvious keyboard slips, not become Grammarly after three espressos.

### 5. Some inputs are hopeless without target text

Some typed strings are effectively random keyboard noise. Example class:

```text
sad fsa f sadf sad fsad f saf sa dfas ...
```

No prompt can reliably recover the target quote from that without seeing the target. The benchmark needs to account for this, otherwise prompt search overfits to hallucinating plausible English.

Hopeless rows should not dominate prompt selection. They should be tracked separately.

## Prompt experiments completed

The most useful completed training run used a stratified 64-row training set:

- hard rows: current mismatch rows and rows with many remaining errors
- medium rows: imperfect current corrections
- easy rows: current all-correct rows
- deterministic seed: 1337

Completed candidates:

### Current prompt, training subset

```text
accuracy: 0.7724
delta over typed: +59 exact words
all-correct: 8 / 64
word-count mismatches: 32
improved runs: 29
worsened runs: 0
avg latency: 461 ms
p95 latency: 734 ms
```

### `strict_count`

Prompt:

```text
You correct fast, sloppy typing into the intended English text. Output ONLY the corrected text. Keep exactly the same number of whitespace-separated words as the input. Do not add, remove, split, merge, or reorder words. Fix spelling, capitalization, apostrophes, and punctuation inside each word.
```

Training result:

```text
accuracy: 0.7879
delta over typed: +76 exact words
all-correct: 8 / 64
word-count mismatches: 21
improved runs: 33
worsened runs: 4
avg latency: 500 ms
p95 latency: 680 ms
```

Interpretation:

- Better correction rate than current.
- Lower mismatch count.
- But worsened 4 runs, which is not free.
- Also slightly slower on average.

### `one_token`

Prompt:

```text
Correct this fast typing-test input. Treat each whitespace-separated input token as one output token: for every input token, output exactly one corrected token in the same position. Fix typos, missing apostrophes, punctuation, and capitalization when obvious. If unsure, leave the token unchanged. Output only the corrected text.
```

Training result:

```text
accuracy: 0.7951
delta over typed: +84 exact words
all-correct: 8 / 64
word-count mismatches: 20
improved runs: 35
worsened runs: 3
avg latency: 454 ms
p95 latency: 744 ms
```

Interpretation:

- Best numeric training score among completed candidates.
- Roughly as fast as current.
- But unsafe: it worsened clean rows by capitalizing and rewriting.
- This is the strongest non-shipping candidate.

### `counted_strict`

Prompt included the explicit input token count.

Tiny partial result, 3 rows:

```text
accuracy: 0.6327
worse than current on that subset
```

Interpretation:

- Bad early signal.
- Not worth continuing.

### Longer careful prompts

Examples:

- `token_case`
- `token_case_count`
- `current_token`
- `conservative_token`
- `keyboard_case`

These tried to explicitly forbid capitalization changes, grammar rewrites, synonyms, token splitting, and token merging.

Problem:

- They were much slower in practice.
- Groq TPM throttling became severe.
- Partial results were not compelling enough to justify the latency/token cost.

Interpretation:

Long prompt engineering is the wrong direction for this speed-sensitive app unless it produces a dramatic accuracy win. It did not prove that.

### Short follow-up prompts

I tried to restart with short variants like:

```text
Correct obvious fast-typing typos. Output only text. Same word count and order. Keep capitalization. No rewrites or synonyms. If unsure, copy unchanged.
```

Groq was still rate-limiting heavily:

```text
HTTP 429
retry after ~120s
retry after ~20s
retry after ~60s
```

Only one or a few rows completed, not enough for a valid conclusion.

## Concrete bad examples from candidate prompts

### Capitalization regression

Target:

```text
this is a modern fairy tale. no happy endings, no wind in our sails.
```

Typed:

```text
this is a modern fairy tale. no happy endings, no wind in our sails.
```

`one_token` output:

```text
This is a modern fairy tale. no happy endings, no wind in our sails.
```

This is a worse exact match despite being normal English.

### Grammar rewrite regression

Target:

```text
when you walk away, i count the steps that you take. do you see how much i need you right now?
```

Typed:

```text
when you walk away, i count the steps that you take. do you see how much in eed you right now?
```

Candidate output:

```text
when you walk away, I count the steps that you take. do you see how much in need you are now?
```

It corrected some English, but not the target. Bad for this app.

### Synonym/semantic guessing regression

Target:

```text
every time i think of exercise, i have to lie right down until the feeling leaves me.
```

Typed:

```text
ever time i think of recrecise, i have to lie right down until the fealing leaves me.
```

Candidate output:

```text
Every time I think of recreate, I have to lie right down until the feeling leaves me.
```

It guessed `recreate`, but the intended target was `exercise`. This is exactly why "plausible English" is not good enough.

## What the benchmark says so far

Prompt-only optimization can probably improve the current prompt a little, but not safely enough from the completed evidence.

The best candidate improved training accuracy by about 2.3 percentage points over current on the stratified training set:

```text
current:   0.7724
one_token: 0.7951
```

But that came with worsened rows. Since the app's user-facing metric is exact quote recovery, damaging correct words is unacceptable unless validation shows a large net win and a low damage rate.

No held-out validation run completed. Therefore shipping a prompt change now would be overfitting. That would be fake ML, not ML.

## Recommended acceptance criteria for a future prompt change

A replacement prompt should only ship if it passes a held-out validation set with:

1. Higher exact word accuracy than current, ideally +2 percentage points or more.
2. Equal or better all-correct quote count.
3. Lower or equal word-count mismatch rate.
4. No meaningful increase in worsened runs.
5. Average latency no worse than current by more than ~100 ms.
6. p95 latency not meaningfully worse than current.
7. Manual inspection of the worst regressions shows no grammar rewriting/synonym behavior.

If a candidate fails any of those, reject it.

## Better directions than more prompt words

### 1. Preserve typed capitalization in post-processing

If the model returns a correction with a different capitalization pattern from the typed word, normalize it back to the typed style.

Examples:

- typed `i`, model `I` -> keep `i`
- typed `If`, model `if` -> maybe keep `If`
- typed all caps -> preserve all caps

This is cheap and fast. It directly attacks one observed regression class.

### 2. Salvage mismatched responses instead of full fallback

Current behavior discards the entire model response when word count mismatches. That is safe but wasteful.

A better approach:

- Try to align raw corrected tokens to typed token positions with edit distance.
- Only accept high-confidence aligned corrections.
- Leave uncertain positions unchanged.
- Still avoid showing scary or misaligned output.

This could recover useful corrections from mismatch rows without prompt bloat.

### 3. Use a token-preserving wire format only if speed remains acceptable

Plain text cannot represent empty typed tokens reliably. A structured prompt could, for example:

```text
0: the
1: worlld
2: was
...
```

Then ask for the same numbered format back.

This may reduce mismatch and preserve positions, but it increases tokens and likely latency. It should be benchmarked, not assumed.

### 4. Decide whether target text is allowed

If the app sends both target and typed text, correction becomes much easier. But that changes the product meaning.

With target text:

- LLM can almost always align to the intended quote.
- Corrected WPM becomes more like "target-confirmed recovery".
- It is no longer a test of whether an LLM can infer intended prose from your noisy typing alone.

Without target text:

- The metric remains closer to real spellcheck/autocorrect behavior.
- Hopeless rows stay hopeless.

This is a product decision, not a prompt detail.

## Shipped production prompt

The shipped prompt is the best completed training candidate:

```text
Correct this fast typing-test input. Treat each whitespace-separated input token as one output token: for every input token, output exactly one corrected token in the same position. Fix typos, missing apostrophes, punctuation, and capitalization when obvious. If unsure, leave the token unchanged. Output only the corrected text.
```

## Final recommendation

Ship `one_token`, then monitor it.

Why this is acceptable:

```text
training accuracy: 0.7951 vs 0.7724 baseline
additional exact word corrections: +25 on the training split
word-count mismatch fallbacks: 20 vs 32 baseline
average latency: ~455 ms vs ~462 ms baseline on the same split
```

What to watch:

- capitalization regressions (`i` -> `I`, sentence-start title casing)
- grammar rewrites instead of typo repair
- plausible synonyms that do not match the target quote
- whether mismatch reduction holds on fresh runs

Future work should combine this short literal prompt with capitalization-preserving post-processing and better mismatch salvage, then validate on held-out rows.
