// Package player plays a random soup of Conway's Game of Life on a terminal.
package player

import (
	"errors"
	"time"
)

// Options is the parsed run configuration. ParseArgs is the only constructor.
// The zero Options is not valid.
type Options struct {
	ok      bool
	seed    int64
	density float64
	width   int
	height  int
	sized   bool
}

// Terminal is the screen and keyboard a run plays on.
type Terminal interface {
	Size() (cols, rows int, err error)
	Draw(f Frame) error
	Keys() <-chan Key
}

// Clock schedules the next generation.
type Clock interface {
	After(d time.Duration) <-chan time.Time
}

// WallClock is the production clock. After returns time.After(d).
type WallClock struct{}

// After returns a channel that fires after d.
func (WallClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// Frame is one screen of the game. Rows are glyph lines, one per grid row.
// Status is the status line, with no padding and no newline.
type Frame struct {
	Rows   []string
	Status string
}

// Key is one decoded keypress.
type Key rune

const (
	// KeyEsc is a lone Escape byte.
	KeyEsc Key = 0x1b
	// KeyCtrlC is the Ctrl-C byte reported in raw mode.
	KeyCtrlC Key = 0x03
)

// UsageError is a bad command line. The message text is not part of the interface.
type UsageError struct {
	Msg string
}

// Error returns the usage message.
func (e *UsageError) Error() string {
	return e.Msg
}

// ErrTerminalTooSmall is returned when the terminal has no room for one cell
// and a status line.
var ErrTerminalTooSmall = errors.New("player: terminal too small")

// ErrDoesNotFit is returned when a requested --size does not fit the terminal.
var ErrDoesNotFit = errors.New("player: grid does not fit the terminal")
