package player

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/attiasas/terminal-game-of-life/life"
)

const (
	liveGlyph = "██"
	deadGlyph = "  "
)

// Run plays one game. It panics if opts is the zero Options.
func Run(ctx context.Context, opts Options, term Terminal, clock Clock) error {
	if !opts.ok {
		panic("player: zero Options")
	}
	cols, rows, err := term.Size()
	if err != nil {
		return fmt.Errorf("player: terminal size: %w", err)
	}
	width, height := opts.width, opts.height
	if opts.sized {
		if cols < 2*width || rows < height+1 {
			return ErrDoesNotFit
		}
	} else {
		width = cols / 2
		height = rows - 1
		if width < 1 || height < 1 {
			return ErrTerminalTooSmall
		}
	}
	if err := ctx.Err(); err != nil {
		return nil
	}
	grid := soup(opts.seed, opts.density, width, height)
	gen := uint64(0)
	playing := true
	speed := 10
	keys := term.Keys()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	defer func() {
		close(stop)
		wg.Wait()
	}()

	var tick <-chan time.Time
	abandon := func(ch <-chan time.Time) {
		if ch == nil {
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ch:
			case <-stop:
			case <-ctx.Done():
			}
		}()
	}
	arm := func() {
		abandon(tick)
		tick = nil
		if !playing {
			return
		}
		tick = clock.After(time.Second / time.Duration(speed))
	}

	arm()
	if err := draw(term, grid, gen, playing, speed); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case key, ok := <-keys:
			if !ok {
				return nil
			}
			if key == 'q' || key == KeyEsc || key == KeyCtrlC {
				return nil
			}
			switch key {
			case ' ':
				playing = !playing
			case 'n':
				grid = life.Next(grid)
				gen++
				playing = false
			case '+':
				if speed < 60 {
					speed++
				}
			case '-':
				if speed > 1 {
					speed--
				}
			default:
				continue
			}
			arm()
			if err := draw(term, grid, gen, playing, speed); err != nil {
				return err
			}
		case <-tick:
			grid = life.Next(grid)
			gen++
			tick = nil
			arm()
			if err := draw(term, grid, gen, playing, speed); err != nil {
				return err
			}
		}
	}
}

func draw(term Terminal, grid [][]bool, gen uint64, playing bool, speed int) error {
	if err := term.Draw(render(grid, gen, playing, speed)); err != nil {
		return fmt.Errorf("player: draw: %w", err)
	}
	return nil
}

func soup(seed int64, density float64, width, height int) [][]bool {
	r := rand.New(rand.NewPCG(uint64(seed), 0))
	grid := make([][]bool, height)
	for y := 0; y < height; y++ {
		grid[y] = make([]bool, width)
		for x := 0; x < width; x++ {
			grid[y][x] = r.Float64() < density
		}
	}
	return grid
}

func render(grid [][]bool, gen uint64, playing bool, speed int) Frame {
	rows := make([]string, len(grid))
	for i, row := range grid {
		var b strings.Builder
		for _, cell := range row {
			if cell {
				b.WriteString(liveGlyph)
			} else {
				b.WriteString(deadGlyph)
			}
		}
		rows[i] = b.String()
	}
	state := "paused"
	if playing {
		state = "playing"
	}
	status := fmt.Sprintf("gen %d | %s | %d gen/s | dead border", gen, state, speed)
	return Frame{Rows: rows, Status: status}
}
