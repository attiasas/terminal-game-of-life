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
	assertGrid(t, "", got, want)
}

func TestBlinkerFlipsThenReturnsToTheStart(t *testing.T) {
	start := parse(t, "...\nOOO\n...")
	flipped := parse(t, ".O.\n.O.\n.O.")
	got := life.Next(start)
	assertGrid(t, "generation 1", got, flipped)
	again := life.Next(got)
	assertGrid(t, "generation 2", again, start)
}

func TestGliderShiftsThenSettlesIntoACornerBlock(t *testing.T) {
	start := parse(t, ".O..\n..O.\nOOO.\n....")
	want4 := parse(t, "....\n..O.\n...O\n.OOO")
	want7 := parse(t, "....\n....\n..OO\n..OO")

	got := start
	for range 4 {
		got = life.Next(got)
	}
	assertGrid(t, "generation 4", got, want4)
	for range 3 {
		got = life.Next(got)
	}
	assertGrid(t, "generation 7", got, want7)
	settled := life.Next(got)
	assertGrid(t, "corner block changed", settled, want7)
}

func TestNextDoesNotModifyOrAliasTheInput(t *testing.T) {
	start := parse(t, "OO\nOO")
	got := life.Next(start)
	if !gridsEqual(start, parse(t, "OO\nOO")) {
		t.Fatalf("Next modified the input\n%s", formatGrid(start))
	}
	got[0][0] = !got[0][0]
	if !start[0][0] {
		t.Fatal("Next result aliases the input")
	}
}

func TestNilGridReturnsNil(t *testing.T) {
	if got := life.Next(nil); got != nil {
		t.Fatalf("got\n%s\nwant nil", formatGrid(got))
	}
}

func TestZeroRowsReturnANonNilGrid(t *testing.T) {
	in := [][]bool{}
	got := life.Next(in)
	if got == nil {
		t.Fatal("got nil, want a non-nil empty grid")
	}
	if len(got) != 0 {
		t.Fatalf("got\n%s\nwant an empty grid", formatGrid(got))
	}
}

func TestRaggedGridPanics(t *testing.T) {
	ragged := [][]bool{
		{false, false},
		{false, false, true},
	}
	defer func() {
		if recover() == nil {
			t.Fatal("ragged grid did not panic")
		}
	}()
	life.Next(ragged)
}

func TestZeroLengthRowsReturnANonNilGrid(t *testing.T) {
	in := [][]bool{nil, nil}
	got := life.Next(in)
	if got == nil {
		t.Fatal("got nil, want a non-nil grid")
	}
	if len(got) != len(in) || len(got[0]) != 0 || len(got[1]) != 0 {
		t.Fatalf("got\n%s\nwant two empty rows", formatGrid(got))
	}
}

func TestSameInputReturnsTheSameCellsWithoutSharedState(t *testing.T) {
	start := parse(t, "OO\nOO")
	want := parse(t, "OO\nOO")
	first := life.Next(start)
	first[0][0] = false
	second := life.Next(start)
	assertGrid(t, "Next kept state from an earlier result", second, want)
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

func assertGrid(t *testing.T, label string, got, want [][]bool) {
	t.Helper()
	if gridsEqual(got, want) {
		return
	}
	if label == "" {
		t.Fatalf("got\n%s\nwant\n%s", formatGrid(got), formatGrid(want))
	}
	t.Fatalf("%s\ngot\n%s\nwant\n%s", label, formatGrid(got), formatGrid(want))
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
