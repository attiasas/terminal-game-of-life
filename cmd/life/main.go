package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/attiasas/terminal-game-of-life/internal/tty"
	"github.com/attiasas/terminal-game-of-life/player"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses args, plays one game, and restores the terminal on the way out.
// A usage error exits 2. Any other error exits 1. A finished game exits 0.
func run(args []string, in, out *os.File, errOut io.Writer) int {
	opts, err := player.ParseArgs(args)
	if err != nil {
		fmt.Fprintln(errOut, err.Error())
		var usage *player.UsageError
		if errors.As(err, &usage) {
			return 2
		}
		return 1
	}

	terminal, err := tty.Open(in, out)
	if err != nil {
		fmt.Fprintln(errOut, err.Error())
		return 1
	}
	defer terminal.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := player.Run(ctx, opts, terminal, player.WallClock{}); err != nil {
		fmt.Fprintln(errOut, err.Error())
		return 1
	}
	return 0
}
