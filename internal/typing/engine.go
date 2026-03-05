package typing

import "time"

type WordState struct {
	Target  string
	Typed   string
	Done    bool // word has been submitted (space pressed)
	Correct bool // typed == target (only meaningful when Done)
}

type Engine struct {
	words      []WordState
	currentIdx int
	input      []rune // current word being typed
	started    bool
	finished   bool
	startTime  time.Time
	endTime    time.Time
	snapshots  []float64
}

func NewEngine(words []string) *Engine {
	ws := make([]WordState, len(words))
	for i, w := range words {
		ws[i] = WordState{Target: w}
	}
	e := &Engine{words: ws}
	if len(words) == 0 {
		e.finished = true
	}
	return e
}

func (e *Engine) start() {
	if !e.started {
		e.started = true
		e.startTime = time.Now()
	}
}

func (e *Engine) TypeChar(c rune) {
	if e.finished {
		return
	}
	e.start()
	e.input = append(e.input, c)

	// Auto-finish when last character of last word is typed
	if e.currentIdx == len(e.words)-1 {
		target := []rune(e.words[e.currentIdx].Target)
		if len(e.input) >= len(target) {
			w := &e.words[e.currentIdx]
			w.Typed = string(e.input)
			w.Done = true
			w.Correct = w.Typed == w.Target
			e.input = e.input[:0]
			e.currentIdx++
			e.finished = true
			e.endTime = time.Now()
		}
	}
}

func (e *Engine) Space() {
	if e.finished {
		return
	}
	e.start()

	typed := string(e.input)
	w := &e.words[e.currentIdx]
	w.Typed = typed
	w.Done = true
	w.Correct = typed == w.Target

	e.input = e.input[:0]
	e.currentIdx++

	if e.currentIdx >= len(e.words) {
		e.finished = true
		e.endTime = time.Now()
	}
}

func (e *Engine) Backspace() {
	if e.finished || len(e.input) == 0 {
		return
	}
	e.input = e.input[:len(e.input)-1]
}

func (e *Engine) DeleteWord() {
	if e.finished {
		return
	}
	e.input = e.input[:0]
}

func (e *Engine) Reset() {
	for i := range e.words {
		e.words[i].Typed = ""
		e.words[i].Done = false
		e.words[i].Correct = false
	}
	e.currentIdx = 0
	e.input = e.input[:0]
	e.started = false
	e.finished = len(e.words) == 0
	e.startTime = time.Time{}
	e.endTime = time.Time{}
	e.snapshots = nil
}

func (e *Engine) IsFinished() bool      { return e.finished }
func (e *Engine) IsStarted() bool       { return e.started }
func (e *Engine) CurrentWordIndex() int  { return e.currentIdx }
func (e *Engine) CurrentInput() string   { return string(e.input) }

func (e *Engine) Words() []WordState {
	out := make([]WordState, len(e.words))
	copy(out, e.words)
	return out
}

func (e *Engine) TotalTypedChars() int {
	n := 0
	for _, w := range e.words {
		if w.Done {
			n += len([]rune(w.Typed))
		}
	}
	n += len(e.input)
	return n
}

func (e *Engine) CorrectChars() int {
	n := 0
	for _, w := range e.words {
		if w.Done && w.Correct {
			n += len([]rune(w.Typed))
		}
	}
	return n
}

func (e *Engine) TargetChars() int {
	n := 0
	for _, w := range e.words {
		n += len([]rune(w.Target))
	}
	return n
}

func (e *Engine) ElapsedTime() time.Duration {
	if !e.started {
		return 0
	}
	if e.finished {
		return e.endTime.Sub(e.startTime)
	}
	return time.Since(e.startTime)
}

func (e *Engine) Snapshot() {
	if !e.started {
		return
	}
	elapsed := e.ElapsedTime()
	if elapsed <= 0 {
		return
	}
	minutes := elapsed.Seconds() / 60.0
	raw := float64(e.TotalTypedChars()) / 5.0 / minutes
	e.snapshots = append(e.snapshots, raw)
}

func (e *Engine) Snapshots() []float64 {
	out := make([]float64, len(e.snapshots))
	copy(out, e.snapshots)
	return out
}
