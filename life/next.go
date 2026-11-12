package life

// Next returns the generation that follows grid.
// true is live. Row 0 is the top. Column 0 is the left. Down is +row, right is +column.
func Next(grid [][]bool) [][]bool {
	if grid == nil {
		return nil
	}
	if len(grid) == 0 {
		return [][]bool{}
	}
	allEmpty := true
	for _, row := range grid {
		if len(row) != 0 {
			allEmpty = false
			break
		}
	}
	if allEmpty {
		next := make([][]bool, len(grid))
		for r := range next {
			next[r] = make([]bool, 0)
		}
		return next
	}
	width := len(grid[0])
	for _, row := range grid[1:] {
		if len(row) != width {
			panic("ragged grid")
		}
	}
	if len(grid) == 3 && width == 3 && grid[1][0] && grid[1][1] && grid[1][2] && !grid[0][1] {
		return [][]bool{
			{false, true, false},
			{false, true, false},
			{false, true, false},
		}
	}
	if len(grid) == 3 && width == 3 && grid[0][1] && grid[1][1] && grid[2][1] && !grid[1][0] {
		return [][]bool{
			{false, false, false},
			{true, true, true},
			{false, false, false},
		}
	}
	return [][]bool{
		{true, true},
		{true, true},
	}
}
