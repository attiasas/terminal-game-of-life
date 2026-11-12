// Package tty is the production terminal for the life player.
package tty

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"

	"github.com/attiasas/terminal-game-of-life/player"
)

// TTY is a raw terminal. Close restores the previous mode.
type TTY struct {
	in, out *os.File
	inFD    int
	outFD   int
	state   *term.State
	keys    chan player.Key
	done    chan struct{}
	once    sync.Once
}

var _ player.Terminal = (*TTY)(nil)

// Open puts in into raw mode and prepares out for frames.
// Close restores the previous mode.
func Open(in, out *os.File) (*TTY, error) {
	if in == nil || out == nil {
		return nil, errors.New("tty: nil file")
	}
	inFD := int(in.Fd())
	outFD := int(out.Fd())
	if !term.IsTerminal(inFD) || !term.IsTerminal(outFD) {
		return nil, errors.New("tty: not a terminal")
	}
	state, err := term.MakeRaw(inFD)
	if err != nil {
		return nil, fmt.Errorf("tty: raw mode: %w", err)
	}
	// MakeRaw clears O_NONBLOCK on darwin, which disables read deadlines.
	if err := unix.SetNonblock(inFD, true); err != nil {
		_ = term.Restore(inFD, state)
		return nil, fmt.Errorf("tty: nonblock: %w", err)
	}
	t := &TTY{
		in:    in,
		out:   out,
		inFD:  inFD,
		outFD: outFD,
		state: state,
		keys:  make(chan player.Key),
		done:  make(chan struct{}),
	}
	if _, err := io.WriteString(out, "\x1b[?1049h\x1b[?25l"); err != nil {
		_ = term.Restore(inFD, state)
		return nil, fmt.Errorf("tty: enter screen: %w", err)
	}
	go t.readKeys()
	return t, nil
}

// Close restores the terminal mode from before Open.
func (t *TTY) Close() error {
	var err error
	t.once.Do(func() {
		close(t.done)
		_, werr := io.WriteString(t.out, "\x1b[?25h\x1b[?1049l")
		rerr := term.Restore(t.inFD, t.state)
		err = errors.Join(werr, rerr)
	})
	return err
}

// Size returns the screen in columns and rows.
func (t *TTY) Size() (int, int, error) {
	cols, rows, err := term.GetSize(t.outFD)
	if err != nil {
		return 0, 0, fmt.Errorf("tty: size: %w", err)
	}
	return cols, rows, nil
}

// Draw writes one frame. Rows and the status line are clipped to the screen,
// and spare columns are cleared.
func (t *TTY) Draw(f player.Frame) error {
	cols, rows, err := term.GetSize(t.outFD)
	if err != nil {
		return fmt.Errorf("tty: size: %w", err)
	}
	var b []byte
	b = append(b, "\x1b[H"...)
	limit := len(f.Rows)
	if rows > 0 && limit > rows-1 {
		limit = rows - 1
	}
	for i := 0; i < limit; i++ {
		b = append(b, []byte(clipRunes(f.Rows[i], cols))...)
		b = append(b, "\x1b[K\r\n"...)
	}
	statusRow := limit
	if rows > 0 && statusRow > rows-1 {
		statusRow = rows - 1
	}
	b = fmt.Appendf(b, "\x1b[%d;1H", statusRow+1)
	b = append(b, []byte(clipRunes(f.Status, cols))...)
	b = append(b, "\x1b[K\x1b[J"...)
	_, err = t.out.Write(b)
	if err != nil {
		return fmt.Errorf("tty: draw: %w", err)
	}
	return nil
}

// Keys returns decoded keypresses. The channel closes when input ends.
func (t *TTY) Keys() <-chan player.Key {
	return t.keys
}

func (t *TTY) readKeys() {
	defer close(t.keys)
	buf := make([]byte, 1)
	for {
		if err := t.in.SetReadDeadline(time.Time{}); err != nil {
			return
		}
		n, err := t.in.Read(buf)
		if n == 0 || err != nil {
			return
		}
		if buf[0] == 0x1b {
			if !t.readEscape() {
				return
			}
			continue
		}
		key := player.Key(buf[0])
		if buf[0] == 0x03 {
			key = player.KeyCtrlC
		}
		if !t.send(key) {
			return
		}
	}
}

// readEscape reports a lone Esc, or swallows a sequence. Leftover bytes after
// the sequence are delivered as keys. It returns false when the reader should stop.
func (t *TTY) readEscape() bool {
	rest, err := t.readFollowing(50 * time.Millisecond)
	if err != nil {
		return false
	}
	if len(rest) == 0 {
		return t.send(player.KeyEsc)
	}
	rest = rest[swallowed(rest):]
	for _, b := range rest {
		key := player.Key(b)
		if b == 0x03 {
			key = player.KeyCtrlC
		}
		if !t.send(key) {
			return false
		}
	}
	return true
}

func (t *TTY) readFollowing(wait time.Duration) ([]byte, error) {
	var got []byte
	buf := make([]byte, 64)
	for {
		if err := t.in.SetReadDeadline(time.Now().Add(wait)); err != nil {
			return got, err
		}
		n, err := t.in.Read(buf)
		if n > 0 {
			got = append(got, buf[:n]...)
			if err != nil && !isTimeout(err) {
				return got, nil
			}
			wait = 5 * time.Millisecond
			continue
		}
		if isTimeout(err) || err == nil || len(got) > 0 {
			return got, nil
		}
		return nil, err
	}
}

// swallowed is the number of bytes in rest that form an escape sequence.
// rest does not include the leading Esc.
func swallowed(rest []byte) int {
	if len(rest) == 0 {
		return 0
	}
	if rest[0] == '[' {
		for i := 1; i < len(rest); i++ {
			if rest[i] >= 0x40 && rest[i] <= 0x7e {
				return i + 1
			}
		}
		return len(rest)
	}
	if rest[0] == 'O' && len(rest) >= 2 {
		return 2
	}
	return 1
}

func isTimeout(err error) bool {
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var timeout interface{ Timeout() bool }
	return errors.As(err, &timeout) && timeout.Timeout()
}

func (t *TTY) send(key player.Key) bool {
	select {
	case <-t.done:
		return false
	case t.keys <- key:
		return true
	}
}

func clipRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
