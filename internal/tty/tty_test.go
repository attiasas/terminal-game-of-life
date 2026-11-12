package tty_test

import (
	"bytes"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/attiasas/terminal-game-of-life/internal/tty"
	"github.com/attiasas/terminal-game-of-life/player"
)

func TestOpenRejectsAPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		r.Close()
		w.Close()
	})

	if _, err := tty.Open(r, w); err == nil {
		t.Fatal("Open on a pipe returned nil")
	}
}

func TestCtrlCIsReported(t *testing.T) {
	master, slave := openPTY(t)
	term, err := tty.Open(slave, slave)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { term.Close() })

	if _, err := master.Write([]byte{0x03}); err != nil {
		t.Fatal(err)
	}
	select {
	case key := <-term.Keys():
		if key != player.KeyCtrlC {
			t.Fatalf("key = %q, want Ctrl-C", key)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Ctrl-C")
	}
}

func TestDrawClipsToTheScreenAndClearsSpareColumns(t *testing.T) {
	master, slave := openPTY(t)
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 4, Col: 10}); err != nil {
		t.Fatal(err)
	}
	term, err := tty.Open(slave, slave)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { term.Close() })

	err = term.Draw(player.Frame{
		Rows:   []string{"██"},
		Status: "gen 0 | playing | 10 gen/s | dead border",
	})
	if err != nil {
		t.Fatal(err)
	}
	output := readAvailable(t, master)
	if !strings.Contains(output, "██") {
		t.Fatalf("output missing the live cell: %q", output)
	}
	if !strings.Contains(output, "\x1b[K") {
		t.Fatalf("output did not clear spare columns: %q", output)
	}
	if !strings.Contains(output, "gen 0 | pl") {
		t.Fatalf("output missing the clipped status: %q", output)
	}
	if strings.Contains(output, "dead border") {
		t.Fatalf("status was not clipped: %q", output)
	}
}

func readAvailable(t *testing.T, f *os.File) string {
	t.Helper()
	fd := int(f.Fd())
	if err := unix.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	pfd := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	nready, err := unix.Poll(pfd, 200)
	if err != nil {
		t.Fatal(err)
	}
	if nready == 0 || pfd[0].Revents&unix.POLLIN == 0 {
		t.Fatalf("no terminal output (n=%d revents=%#x)", nready, pfd[0].Revents)
	}
	var b bytes.Buffer
	buf := make([]byte, 4096)
	for {
		n, err := syscall.Read(fd, buf)
		if n > 0 {
			b.Write(buf[:n])
			continue
		}
		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
			break
		}
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		break
	}
	return b.String()
}

func TestCloseRestoresTheTerminalMode(t *testing.T) {
	_, slave := openPTY(t)
	fd := int(slave.Fd())
	before, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		t.Fatal(err)
	}
	term, err := tty.Open(slave, slave)
	if err != nil {
		t.Fatal(err)
	}
	if err := term.Close(); err != nil {
		t.Fatal(err)
	}
	if err := term.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		t.Fatal(err)
	}
	if *before != *after {
		t.Fatalf("termios not restored\nbefore: %#v\nafter: %#v", *before, *after)
	}
}

func TestLoneEscIsReported(t *testing.T) {
	master, slave := openPTY(t)
	term, err := tty.Open(slave, slave)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { term.Close() })

	if _, err := master.Write([]byte{0x1b}); err != nil {
		t.Fatal(err)
	}
	select {
	case key := <-term.Keys():
		if key != player.KeyEsc {
			t.Fatalf("key = %#x, want Esc", key)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Esc")
	}
}

func TestEscapeSequenceProducesNoKey(t *testing.T) {
	master, slave := openPTY(t)
	term, err := tty.Open(slave, slave)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { term.Close() })

	if _, err := master.Write([]byte("\x1b[Aq")); err != nil {
		t.Fatal(err)
	}
	select {
	case key := <-term.Keys():
		if key != 'q' {
			t.Fatalf("key = %#x, want q", key)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for q")
	}
}

func openPTY(t *testing.T) (master, slave *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { master.Close() })

	if err := unix.IoctlSetInt(int(master.Fd()), unix.TIOCPTYGRANT, 0); err != nil {
		t.Fatal(err)
	}
	if err := unix.IoctlSetInt(int(master.Fd()), unix.TIOCPTYUNLK, 0); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 128)
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		master.Fd(),
		uintptr(unix.TIOCPTYGNAME),
		uintptr(unsafe.Pointer(&buf[0])),
	)
	if errno != 0 {
		t.Fatal(errno)
	}
	if i := bytes.IndexByte(buf, 0); i >= 0 {
		buf = buf[:i]
	}
	slave, err = os.OpenFile(string(buf), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { slave.Close() })
	return master, slave
}
