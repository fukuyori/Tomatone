package main

import "time"

type Phase int

const (
	PhaseFocus Phase = iota
	PhaseShortBreak
	PhaseLongBreak
)

type Pomodoro struct {
	focusDuration            time.Duration
	shortBreakDuration       time.Duration
	longBreakDuration        time.Duration
	sessionsBeforeLongBreak  int
	phase                    Phase
	remaining                time.Duration
	running                  bool
	focusSessionsInCycle     int
	completedFocusSessions   int
	lastTransitionWasNatural bool
}

func NewPomodoro(cfg RuntimeConfig) *Pomodoro {
	return &Pomodoro{
		focusDuration:           cfg.Focus,
		shortBreakDuration:      cfg.ShortBreak,
		longBreakDuration:       cfg.LongBreak,
		sessionsBeforeLongBreak: cfg.FocusSessionsBeforeLong,
		phase:                   PhaseFocus,
		remaining:               cfg.Focus,
		running:                 cfg.AutoStart,
	}
}

func (p *Pomodoro) Tick(elapsed time.Duration) bool {
	if !p.running || elapsed <= 0 {
		return false
	}
	transitioned := false
	for elapsed >= p.remaining {
		elapsed -= p.remaining
		p.advance(true)
		transitioned = true
	}
	p.remaining -= elapsed
	return transitioned
}

func (p *Pomodoro) TogglePause() {
	p.running = !p.running
}

func (p *Pomodoro) Reset() {
	p.remaining = p.durationFor(p.phase)
}

func (p *Pomodoro) Skip() {
	p.advance(false)
}

func (p *Pomodoro) advance(natural bool) {
	previous := p.phase
	switch p.phase {
	case PhaseFocus:
		if natural {
			p.focusSessionsInCycle++
			p.completedFocusSessions++
		}
		if natural && p.focusSessionsInCycle >= p.sessionsBeforeLongBreak {
			p.phase = PhaseLongBreak
		} else {
			p.phase = PhaseShortBreak
		}
	case PhaseShortBreak:
		p.phase = PhaseFocus
	case PhaseLongBreak:
		p.focusSessionsInCycle = 0
		p.phase = PhaseFocus
	}
	p.remaining = p.durationFor(p.phase)
	p.lastTransitionWasNatural = natural && previous != p.phase
}

func (p *Pomodoro) durationFor(phase Phase) time.Duration {
	switch phase {
	case PhaseShortBreak:
		return p.shortBreakDuration
	case PhaseLongBreak:
		return p.longBreakDuration
	default:
		return p.focusDuration
	}
}

func (p *Pomodoro) Phase() Phase                 { return p.phase }
func (p *Pomodoro) Remaining() time.Duration     { return p.remaining }
func (p *Pomodoro) Running() bool                { return p.running }
func (p *Pomodoro) Completed() int               { return p.completedFocusSessions }
func (p *Pomodoro) SessionsBeforeLongBreak() int { return p.sessionsBeforeLongBreak }

func (p *Pomodoro) SessionNumber() int {
	if p.phase == PhaseLongBreak {
		return p.sessionsBeforeLongBreak
	}
	n := p.focusSessionsInCycle + 1
	if n > p.sessionsBeforeLongBreak {
		return p.sessionsBeforeLongBreak
	}
	return n
}

func (p *Pomodoro) Progress() float64 {
	total := p.durationFor(p.phase)
	if total <= 0 {
		return 0
	}
	progress := 1 - float64(p.remaining)/float64(total)
	if progress < 0 {
		return 0
	}
	if progress > 1 {
		return 1
	}
	return progress
}

func (p *Pomodoro) NextPhase() Phase {
	switch p.phase {
	case PhaseFocus:
		if p.focusSessionsInCycle+1 >= p.sessionsBeforeLongBreak {
			return PhaseLongBreak
		}
		return PhaseShortBreak
	default:
		return PhaseFocus
	}
}
