// Package life advances one generation of Conway's Game of Life on a finite grid.
package life

// Next returns the generation that follows grid.
// true is live. Row 0 is the top. Column 0 is the left. Down is +row, right is +column.
func Next(grid [][]bool) [][]bool {
	if grid == nil {
		return nil
	}

	rows := len(grid)
	cols := 0
	if rows > 0 {
		cols = len(grid[0])
	}
	for r := 1; r < rows; r++ {
		if len(grid[r]) != cols {
			panic("ragged grid")
		}
	}

	next := make([][]bool, rows)
	for r := 0; r < rows; r++ {
		next[r] = make([]bool, cols)
		for c := 0; c < cols; c++ {
			neighbors := liveNeighbors(grid, r, c, rows, cols)
			next[r][c] = neighbors == 3 || (neighbors == 2 && grid[r][c])
		}
	}
	return next
}

// liveNeighbors counts the eight cells around (row, col). A neighbor outside
// the rectangle is dead.
func liveNeighbors(grid [][]bool, row, col, rows, cols int) int {
	live := 0
	for dr := -1; dr <= 1; dr++ {
		for dc := -1; dc <= 1; dc++ {
			if dr == 0 && dc == 0 {
				continue
			}
			r, c := row+dr, col+dc
			if r < 0 || c < 0 || r >= rows || c >= cols {
				continue
			}
			if grid[r][c] {
				live++
			}
		}
	}
	return live
}
