package main

import (
	"bytes"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

func TestUsageExits2(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"--help"}, nil, nil, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr = %q", code, stderr.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("usage error produced no message")
	}
}

func TestBadFlagExits2(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"--density", "9"}, nil, nil, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr = %q", code, stderr.String())
	}
}

func TestNotATerminalExits1(t *testing.T) {
	in, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, out, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		in.Close()
		inW.Close()
		out.Close()
		outR.Close()
	})

	var stderr bytes.Buffer
	code := run(nil, in, out, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr = %q", code, stderr.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("open failure produced no message")
	}
}

func TestGridThatDoesNotFitExits1(t *testing.T) {
	_, slave := openPTY(t)
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 5, Col: 10}); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	code := run([]string{"--seed", "1", "--size", "40x20"}, slave, slave, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "does not fit") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestQuitExits0AndRestoresTheTerminal(t *testing.T) {
	master, slave := openPTY(t)
	fd := int(slave.Fd())
	if err := unix.IoctlSetWinsize(fd, unix.TIOCSWINSZ, &unix.Winsize{Row: 24, Col: 80}); err != nil {
		t.Fatal(err)
	}
	before, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		t.Fatal(err)
	}

	codeCh := make(chan int, 1)
	var stderr bytes.Buffer
	go func() {
		codeCh <- run([]string{"--seed", "1", "--density", "0", "--size", "4x3"}, slave, slave, &stderr)
	}()
	if output := readAvailable(t, master); !strings.Contains(output, "gen 0") {
		t.Fatalf("screen = %q", output)
	}
	if _, err := master.Write([]byte{'q'}); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-codeCh:
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr = %q", code, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for quit")
	}
	after, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		t.Fatal(err)
	}
	if *before != *after {
		t.Fatal("terminal mode was not restored")
	}
}

func TestSignalExits0(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			master, slave := openPTY(t)
			if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 24, Col: 80}); err != nil {
				t.Fatal(err)
			}
			codeCh := make(chan int, 1)
			var stderr bytes.Buffer
			go func() {
				codeCh <- run([]string{"--seed", "1", "--density", "0", "--size", "4x3"}, slave, slave, &stderr)
			}()
			if output := readAvailable(t, master); !strings.Contains(output, "gen 0") {
				t.Fatalf("screen = %q", output)
			}
			if err := syscall.Kill(os.Getpid(), sig); err != nil {
				t.Fatal(err)
			}
			select {
			case code := <-codeCh:
				if code != 0 {
					t.Fatalf("exit = %d, want 0; stderr = %q", code, stderr.String())
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("timed out waiting for %s", sig)
			}
		})
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
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), uintptr(unix.TIOCPTYGNAME), uintptr(unsafe.Pointer(&buf[0])))
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

func readAvailable(t *testing.T, f *os.File) string {
	t.Helper()
	fd := int(f.Fd())
	if err := unix.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	buf := make([]byte, 4096)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		n, err := syscall.Read(fd, buf)
		if n > 0 {
			b.Write(buf[:n])
			if strings.Contains(b.String(), "gen 0") {
				return b.String()
			}
			continue
		}
		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
			pfd := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
			if _, perr := unix.Poll(pfd, 50); perr != nil {
				t.Fatal(perr)
			}
			continue
		}
		if err != nil {
			t.Fatalf("read: %v", err)
		}
	}
	return b.String()
}
