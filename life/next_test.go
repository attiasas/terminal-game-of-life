package life_test

import (
	"strings"
	"testing"

	"github.com/attiasas/terminal-game-of-life/life"
)

func TestBlockIsUnchangedAfterOneGeneration(t *testing.T) {
	start := parse(t, "OO\nOO")
	got := life.Next(start)
	want := parse(t, "OO\nOO")
	if !gridsEqual(got, want) {
		t.Fatalf("got\n%s\nwant\n%s", formatGrid(got), formatGrid(want))
	}
}

func parse(t *testing.T, picture string) [][]bool {
	t.Helper()
	picture = strings.TrimSpace(picture)
	lines := strings.Split(picture, "\n")
	grid := make([][]bool, len(lines))
	for r, line := range lines {
		grid[r] = make([]bool, len(line))
		for c, ch := range line {
			switch ch {
			case 'O':
				grid[r][c] = true
			case '.':
				grid[r][c] = false
			default:
				t.Fatalf("bad cell %q in %q", string(ch), line)
			}
		}
	}
	return grid
}

func gridsEqual(a, b [][]bool) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for r := range a {
		if len(a[r]) != len(b[r]) {
			return false
		}
		for c := range a[r] {
			if a[r][c] != b[r][c] {
				return false
			}
		}
	}
	return true
}

func formatGrid(grid [][]bool) string {
	if grid == nil {
		return "<nil>"
	}
	var b strings.Builder
	for r, row := range grid {
		if r > 0 {
			b.WriteByte('\n')
		}
		if row == nil {
			b.WriteString("<nil row>")
			continue
		}
		for _, cell := range row {
			if cell {
				b.WriteByte('O')
			} else {
				b.WriteByte('.')
			}
		}
	}
	return b.String()
}
