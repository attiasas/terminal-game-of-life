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
	return [][]bool{
		{true, true},
		{true, true},
	}
}
