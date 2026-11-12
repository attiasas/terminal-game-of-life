package player_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/attiasas/terminal-game-of-life/player"
)

func TestHelpReturnsUsageError(t *testing.T) {
	assertUsage(t, []string{"--help"})
}

func TestUnknownFlagReturnsUsageError(t *testing.T) {
	assertUsage(t, []string{"--wrap"})
}

func TestPositionalArgumentReturnsUsageError(t *testing.T) {
	assertUsage(t, []string{"pattern.rle"})
}

func TestDensityAboveOneReturnsUsageError(t *testing.T) {
	assertUsage(t, []string{"--density", "1.1"})
}

func TestDensityNaNReturnsUsageError(t *testing.T) {
	assertUsage(t, []string{"--density", "NaN"})
}

func TestBadSizeReturnsUsageError(t *testing.T) {
	assertUsage(t, []string{"--size", "0x1"})
}

func TestOtherBadFlagsReturnUsageError(t *testing.T) {
	cases := [][]string{
		{"--size", "01x1"},
		{"--size", "1X1"},
		{"--size", "10x0"},
		{"--size", "10"},
		{"--density", "-0.01"},
		{"--density", "Inf"},
		{"--density", "2"},
		{"--seed", "nope"},
		{"--seed", "1.5"},
		{"--seed", "1", "extra"},
		{"-h"},
		{"--density"},
	}
	for _, args := range cases {
		t.Run(fmtArgs(args), func(t *testing.T) {
			assertUsage(t, args)
		})
	}
}

func TestValidFlagsParse(t *testing.T) {
	cases := [][]string{
		{"--seed", "0", "--density", "0", "--size", "1x1"},
		{"--seed", "-42", "--density", "1", "--size", "8x3"},
		{"--seed", "9223372036854775807"},
		{"--seed", "-9223372036854775808"},
		{"--density", "0.5", "--size", "2x2"},
		nil,
	}
	for _, args := range cases {
		t.Run(fmtArgs(args), func(t *testing.T) {
			if _, err := player.ParseArgs(args); err != nil {
				t.Fatalf("ParseArgs(%q) = %v", args, err)
			}
		})
	}
}

func fmtArgs(args []string) string {
	if args == nil {
		return "nil"
	}
	return strings.Join(args, " ")
}

func assertUsage(t *testing.T, args []string) {
	t.Helper()
	_, err := player.ParseArgs(args)
	var usage *player.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("ParseArgs(%q) err = %v, want *UsageError", args, err)
	}
}
