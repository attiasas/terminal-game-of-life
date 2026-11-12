package player_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/attiasas/terminal-game-of-life/life"
	"github.com/attiasas/terminal-game-of-life/player"
)

func TestRunStartsPlayingAndATickAdvancesOneGeneration(t *testing.T) {
	s := startRun(t, []string{"--seed", "1", "--density", "0.30", "--size", "8x6"}, 40, 20)
	frame := s.frame(t)
	if frame.Status != "gen 0 | playing | 10 gen/s | dead border" {
		t.Fatalf("status = %q", frame.Status)
	}
	if got := s.clock.Delay(); got != time.Second/10 {
		t.Fatalf("delay = %v, want %v", got, time.Second/10)
	}
	if !s.clock.Fire() {
		t.Fatal("tick was not armed")
	}
	next := s.frame(t)
	if next.Status != "gen 1 | playing | 10 gen/s | dead border" {
		t.Fatalf("status = %q", next.Status)
	}
	if got := s.clock.Delay(); got != time.Second/10 {
		t.Fatalf("delay after tick = %v, want %v", got, time.Second/10)
	}
	s.cancel()
	s.wait(t)
}

func TestSpacePausesAndResumes(t *testing.T) {
	s := startRun(t, []string{"--seed", "1", "--density", "0", "--size", "4x3"}, 20, 10)
	first := s.frame(t)
	calls := s.clock.Calls()

	s.key(t, ' ')
	paused := s.frame(t)
	if paused.Status != "gen 0 | paused | 10 gen/s | dead border" {
		t.Fatalf("status = %q", paused.Status)
	}
	if !sameRows(first, paused) {
		t.Fatal("pause changed the grid")
	}
	if s.clock.Calls() != calls {
		t.Fatalf("pause armed the clock (%d calls, was %d)", s.clock.Calls(), calls)
	}
	if !s.clock.Fire() {
		t.Fatal("abandoned tick was not delivered")
	}

	s.key(t, ' ')
	resumed := s.frame(t)
	if resumed.Status != "gen 0 | playing | 10 gen/s | dead border" {
		t.Fatalf("status = %q", resumed.Status)
	}
	if !sameRows(first, resumed) {
		t.Fatal("resume changed the grid")
	}
	if s.clock.Calls() != calls+1 {
		t.Fatalf("resume calls = %d, want %d", s.clock.Calls(), calls+1)
	}
	if got := s.clock.Delay(); got != time.Second/10 {
		t.Fatalf("delay = %v, want %v", got, time.Second/10)
	}
	s.cancel()
	s.wait(t)
}

func TestNAdvancesOneGenerationAndStaysPaused(t *testing.T) {
	s := startRun(t, []string{"--seed", "7", "--density", "1", "--size", "5x4"}, 40, 12)
	first := s.frame(t)
	calls := s.clock.Calls()
	before := gridFromFrame(t, first)

	s.key(t, 'n')
	stepped := s.frame(t)
	if stepped.Status != "gen 1 | paused | 10 gen/s | dead border" {
		t.Fatalf("status = %q", stepped.Status)
	}
	got := gridFromFrame(t, stepped)
	want := life.Next(before)
	if gridsEqual(before, want) {
		t.Fatal("fixture did not change under life.Next")
	}
	assertGrid(t, got, want)
	if s.clock.Calls() != calls {
		t.Fatal("n armed the clock")
	}
	if !s.clock.Fire() {
		t.Fatal("abandoned tick was not delivered")
	}

	s.key(t, 'n')
	again := s.frame(t)
	if again.Status != "gen 2 | paused | 10 gen/s | dead border" {
		t.Fatalf("status = %q", again.Status)
	}
	assertGrid(t, gridFromFrame(t, again), life.Next(want))
	if s.clock.Calls() != calls {
		t.Fatal("n while paused armed the clock")
	}
	s.cancel()
	s.wait(t)
}

func TestPlusRaisesTheSpeedAndArmsTheNewDelay(t *testing.T) {
	s := startRun(t, []string{"--seed", "1", "--density", "0", "--size", "3x2"}, 20, 8)
	first := s.frame(t)
	s.key(t, '+')
	next := s.frame(t)
	if next.Status != "gen 0 | playing | 11 gen/s | dead border" {
		t.Fatalf("status = %q", next.Status)
	}
	if !sameRows(first, next) {
		t.Fatal("+ changed the grid")
	}
	if got := s.clock.Delay(); got != time.Second/11 {
		t.Fatalf("delay = %v, want %v", got, time.Second/11)
	}
	if s.clock.Calls() != 2 {
		t.Fatalf("After calls = %d, want 2", s.clock.Calls())
	}
	s.cancel()
	s.wait(t)
}

func TestMinusLowersTheSpeedAndArmsTheNewDelay(t *testing.T) {
	s := startRun(t, []string{"--seed", "1", "--density", "0", "--size", "3x2"}, 20, 8)
	s.frame(t)
	s.key(t, '-')
	next := s.frame(t)
	if next.Status != "gen 0 | playing | 9 gen/s | dead border" {
		t.Fatalf("status = %q", next.Status)
	}
	if got := s.clock.Delay(); got != time.Second/9 {
		t.Fatalf("delay = %v, want %v", got, time.Second/9)
	}
	s.cancel()
	s.wait(t)
}

func TestPlusAndMinusClampSpeedBetween1And60(t *testing.T) {
	s := startRun(t, []string{"--seed", "1", "--density", "0", "--size", "3x2"}, 20, 8)
	s.frame(t)

	var frame player.Frame
	for range 50 {
		s.key(t, '+')
		frame = s.frame(t)
	}
	if frame.Status != "gen 0 | playing | 60 gen/s | dead border" {
		t.Fatalf("after fifty pluses, status = %q", frame.Status)
	}
	if got := s.clock.Delay(); got != time.Second/60 {
		t.Fatalf("delay at 60 = %v, want %v", got, time.Second/60)
	}
	calls := s.clock.Calls()
	s.key(t, '+')
	frame = s.frame(t)
	if frame.Status != "gen 0 | playing | 60 gen/s | dead border" {
		t.Fatalf("plus at 60, status = %q", frame.Status)
	}
	if s.clock.Calls() != calls+1 {
		t.Fatalf("plus at 60 calls = %d, want %d", s.clock.Calls(), calls+1)
	}
	if got := s.clock.Delay(); got != time.Second/60 {
		t.Fatalf("delay still at 60 = %v", got)
	}

	for range 59 {
		s.key(t, '-')
		frame = s.frame(t)
	}
	if frame.Status != "gen 0 | playing | 1 gen/s | dead border" {
		t.Fatalf("after fifty-nine minuses, status = %q", frame.Status)
	}
	if got := s.clock.Delay(); got != time.Second/1 {
		t.Fatalf("delay at 1 = %v, want %v", got, time.Second)
	}
	calls = s.clock.Calls()
	s.key(t, '-')
	frame = s.frame(t)
	if frame.Status != "gen 0 | playing | 1 gen/s | dead border" {
		t.Fatalf("minus at 1, status = %q", frame.Status)
	}
	if s.clock.Calls() != calls+1 {
		t.Fatalf("minus at 1 calls = %d, want %d", s.clock.Calls(), calls+1)
	}
	if got := s.clock.Delay(); got != time.Second {
		t.Fatalf("delay still at 1 = %v", got)
	}
	s.cancel()
	s.wait(t)
}

func TestQQuitsWithoutAFrame(t *testing.T) {
	assertQuitDrawsNothing(t, 'q')
}

func TestEscQuitsWithoutAFrame(t *testing.T) {
	assertQuitDrawsNothing(t, player.KeyEsc)
}

func TestCtrlCQuitsWithoutAFrame(t *testing.T) {
	assertQuitDrawsNothing(t, player.KeyCtrlC)
}

func assertQuitDrawsNothing(t *testing.T, key player.Key) {
	t.Helper()
	s := startRun(t, []string{"--seed", "1", "--density", "0", "--size", "3x2"}, 20, 8)
	s.frame(t)
	s.key(t, key)
	if err := s.wait(t); err != nil {
		t.Fatal(err)
	}
	select {
	case frame := <-s.term.frames:
		t.Fatalf("quit drew a frame: %#v", frame)
	default:
	}
}

func TestNoSizeUsesTerminalCellsAndKeepsAStatusRow(t *testing.T) {
	s := startRun(t, nil, 81, 24)
	frame := s.frame(t)
	if frame.Status != "gen 0 | playing | 10 gen/s | dead border" {
		t.Fatalf("status = %q", frame.Status)
	}
	if len(frame.Rows) != 23 {
		t.Fatalf("rows = %d, want 23", len(frame.Rows))
	}
	for i, row := range frame.Rows {
		if utf8.RuneCountInString(row) != 80 {
			t.Fatalf("row %d has %d runes, want 80", i, utf8.RuneCountInString(row))
		}
		assertCells(t, row)
	}
	s.cancel()
	s.wait(t)
}

func TestTerminalTooSmallDrawsNothing(t *testing.T) {
	assertRunError(t, nil, 1, 1, player.ErrTerminalTooSmall)
	assertRunError(t, nil, 1, 24, player.ErrTerminalTooSmall)
	assertRunError(t, nil, 80, 1, player.ErrTerminalTooSmall)
}

func TestGridThatDoesNotFitDrawsNothing(t *testing.T) {
	args := []string{"--seed", "1", "--size", "10x5"}
	assertRunError(t, args, 19, 6, player.ErrDoesNotFit)
	assertRunError(t, args, 20, 5, player.ErrDoesNotFit)
}

func TestExactFitDrawsTheRequestedGrid(t *testing.T) {
	s := startRun(t, []string{"--seed", "1", "--density", "0", "--size", "10x5"}, 20, 6)
	frame := s.frame(t)
	if len(frame.Rows) != 5 {
		t.Fatalf("rows = %d, want 5", len(frame.Rows))
	}
	for i, row := range frame.Rows {
		if utf8.RuneCountInString(row) != 20 {
			t.Fatalf("row %d has %d runes, want 20", i, utf8.RuneCountInString(row))
		}
	}
	s.cancel()
	s.wait(t)
}

func TestZeroOptionsPanics(t *testing.T) {
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		_ = player.Run(context.Background(), player.Options{}, newTerm(20, 10), newClock())
	}()
	select {
	case p := <-done:
		if p == nil {
			t.Fatal("zero Options did not panic")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("zero Options did not panic")
	}
}

func TestMissingSeedIsDrawnAndKeptOffTheStatusLine(t *testing.T) {
	first := firstFrame(t, nil, 80, 24)
	second := firstFrame(t, nil, 80, 24)
	if first.Status != "gen 0 | playing | 10 gen/s | dead border" {
		t.Fatalf("status = %q", first.Status)
	}
	if framesEqual(first, second) {
		t.Fatal("two omitted seeds produced the same soup")
	}
}

func TestAlreadyCancelledContextDrawsNothing(t *testing.T) {
	opts, err := player.ParseArgs([]string{"--seed", "1", "--size", "2x2"})
	if err != nil {
		t.Fatal(err)
	}
	term := newTerm(20, 8)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- player.Run(ctx, opts, term, newClock())
	}()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case frame := <-term.frames:
		t.Fatalf("cancelled run drew a frame: %#v", frame)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the run to stop")
	}
}

func TestDensityZeroIsAllDeadAndOneIsAllLive(t *testing.T) {
	dead := firstFrame(t, []string{"--seed", "3", "--density", "0", "--size", "6x4"}, 40, 12)
	for _, row := range dead.Rows {
		if row != strings.Repeat("  ", 6) {
			t.Fatalf("density 0 row = %q", row)
		}
	}
	live := firstFrame(t, []string{"--seed", "-3", "--density", "1", "--size", "6x4"}, 40, 12)
	for _, row := range live.Rows {
		if row != strings.Repeat("██", 6) {
			t.Fatalf("density 1 row = %q", row)
		}
	}
}

func TestNegativeSeedIsStable(t *testing.T) {
	args := []string{"--seed", "-7", "--density", "0.5", "--size", "9x5"}
	if !framesEqual(firstFrame(t, args, 40, 12), firstFrame(t, args, 40, 12)) {
		t.Fatal("negative seed produced two different soups")
	}
}

func TestIgnoredKeyDoesNotDrawOrDropThePendingTick(t *testing.T) {
	s := startRun(t, []string{"--seed", "1", "--density", "0", "--size", "3x2"}, 20, 8)
	s.frame(t)
	calls := s.clock.Calls()
	delay := s.clock.Delay()
	s.key(t, 'x')
	s.key(t, 'N')
	if s.clock.Calls() != calls {
		t.Fatal("ignored key armed the clock")
	}
	if s.clock.Delay() != delay {
		t.Fatalf("delay = %v, want %v", s.clock.Delay(), delay)
	}
	if !s.clock.Fire() {
		t.Fatal("pending tick was dropped")
	}
	next := s.frame(t)
	if next.Status != "gen 1 | playing | 10 gen/s | dead border" {
		t.Fatalf("status = %q", next.Status)
	}
	s.cancel()
	s.wait(t)
}

func TestClosedKeysEndsTheRun(t *testing.T) {
	s := startRun(t, []string{"--seed", "1", "--density", "0", "--size", "3x2"}, 20, 8)
	s.frame(t)
	close(s.term.keys)
	if err := s.wait(t); err != nil {
		t.Fatal(err)
	}
	select {
	case frame := <-s.term.frames:
		t.Fatalf("closed keys drew a frame: %#v", frame)
	default:
	}
}

func TestCancelEndsTheRunWithoutAnotherFrame(t *testing.T) {
	s := startRun(t, []string{"--seed", "1", "--density", "0", "--size", "3x2"}, 20, 8)
	s.frame(t)
	s.cancel()
	if err := s.wait(t); err != nil {
		t.Fatal(err)
	}
	select {
	case frame := <-s.term.frames:
		t.Fatalf("cancel drew a frame: %#v", frame)
	default:
	}
}

func TestChangingSpeedWhilePausedDoesNotArmTheClock(t *testing.T) {
	s := startRun(t, []string{"--seed", "1", "--density", "0", "--size", "3x2"}, 20, 8)
	s.frame(t)
	s.key(t, ' ')
	s.frame(t)
	calls := s.clock.Calls()
	s.key(t, '+')
	paused := s.frame(t)
	if paused.Status != "gen 0 | paused | 11 gen/s | dead border" {
		t.Fatalf("status = %q", paused.Status)
	}
	if s.clock.Calls() != calls {
		t.Fatal("paused speed change armed the clock")
	}
	s.key(t, '-')
	again := s.frame(t)
	if again.Status != "gen 0 | paused | 10 gen/s | dead border" {
		t.Fatalf("status = %q", again.Status)
	}
	if s.clock.Calls() != calls {
		t.Fatal("paused slowdown armed the clock")
	}
	s.cancel()
	s.wait(t)
}

func TestResumeKeepsTheCurrentSpeed(t *testing.T) {
	s := startRun(t, []string{"--seed", "1", "--density", "0", "--size", "3x2"}, 20, 8)
	s.frame(t)
	s.key(t, '+')
	s.frame(t)
	s.key(t, ' ')
	s.frame(t)
	calls := s.clock.Calls()
	s.key(t, ' ')
	resumed := s.frame(t)
	if resumed.Status != "gen 0 | playing | 11 gen/s | dead border" {
		t.Fatalf("status = %q", resumed.Status)
	}
	if got := s.clock.Delay(); got != time.Second/11 {
		t.Fatalf("delay = %v, want %v", got, time.Second/11)
	}
	if s.clock.Calls() != calls+1 {
		t.Fatalf("After calls = %d, want %d", s.clock.Calls(), calls+1)
	}
	s.cancel()
	s.wait(t)
}

func TestAbandonedTickDoesNotAdvanceTheGeneration(t *testing.T) {
	s := startRun(t, []string{"--seed", "1", "--density", "0", "--size", "3x2"}, 20, 8)
	s.frame(t)
	s.key(t, '+')
	s.frame(t)
	s.fireAbandoned(t)
	select {
	case frame := <-s.term.frames:
		t.Fatalf("abandoned tick drew a frame: %#v", frame)
	default:
	}
	if !s.clock.Fire() {
		t.Fatal("live tick was not armed")
	}
	next := s.frame(t)
	if next.Status != "gen 1 | playing | 11 gen/s | dead border" {
		t.Fatalf("status = %q", next.Status)
	}
	s.cancel()
	s.wait(t)
}

func TestSizeAndDrawErrorsAreWrapped(t *testing.T) {
	opts, err := player.ParseArgs([]string{"--seed", "1", "--size", "2x2"})
	if err != nil {
		t.Fatal(err)
	}
	sizeBoom := errors.New("size broke")
	term := newTerm(20, 8)
	term.sizeErr = sizeBoom
	assertWrapped(t, opts, term, sizeBoom)

	drawBoom := errors.New("draw broke")
	term = newTerm(20, 8)
	term.drawErr = drawBoom
	assertWrapped(t, opts, term, drawBoom)
}

func TestSizeIsCalledOnceBeforeAnyDrawAndKeysOnce(t *testing.T) {
	s := startRun(t, []string{"--seed", "1", "--density", "0", "--size", "3x2"}, 20, 8)
	s.frame(t)
	s.key(t, '+')
	s.frame(t)
	if !s.clock.Fire() {
		t.Fatal("tick was not armed")
	}
	s.frame(t)
	s.key(t, 'q')
	s.wait(t)

	sizeCalls, keyCalls, events := s.term.snapshot()
	if sizeCalls != 1 {
		t.Fatalf("Size calls = %d, want 1", sizeCalls)
	}
	if keyCalls != 1 {
		t.Fatalf("Keys calls = %d, want 1", keyCalls)
	}
	sizeAt, drawAt := -1, -1
	for i, event := range events {
		if event == "size" && sizeAt < 0 {
			sizeAt = i
		}
		if event == "draw" && drawAt < 0 {
			drawAt = i
		}
	}
	if sizeAt < 0 || drawAt < 0 || sizeAt > drawAt {
		t.Fatalf("events = %v", events)
	}
}

func TestWallClockAfterFires(t *testing.T) {
	select {
	case <-player.WallClock{}.After(time.Millisecond):
	case <-time.After(time.Second):
		t.Fatal("wall clock did not fire")
	}
}

func TestSameSeedDensityAndSizeProduceTheSameFirstFrame(t *testing.T) {
	args := []string{"--seed", "42", "--density", "0.30", "--size", "12x8"}
	first := firstFrame(t, args, 80, 24)
	second := firstFrame(t, args, 80, 24)

	if first.Status != "gen 0 | playing | 10 gen/s | dead border" {
		t.Fatalf("status = %q", first.Status)
	}
	if len(first.Rows) != 8 {
		t.Fatalf("rows = %d, want 8", len(first.Rows))
	}
	for i, row := range first.Rows {
		if utf8.RuneCountInString(row) != 24 {
			t.Fatalf("row %d has %d runes, want 24", i, utf8.RuneCountInString(row))
		}
		assertCells(t, row)
	}
	if !framesEqual(first, second) {
		t.Fatalf("first frames differ\n%#v\n%#v", first, second)
	}
}

func assertWrapped(t *testing.T, opts player.Options, term *memTerm, want error) {
	t.Helper()
	errCh := make(chan error, 1)
	go func() {
		errCh <- player.Run(context.Background(), opts, term, newClock())
	}()
	select {
	case err := <-errCh:
		if !errors.Is(err, want) {
			t.Fatalf("err = %v, want wrap of %v", err, want)
		}
	case frame := <-term.frames:
		t.Fatalf("drew a frame: %#v", frame)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the run to stop")
	}
}

func assertRunError(t *testing.T, args []string, cols, rows int, want error) {
	t.Helper()
	opts, err := player.ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	term := newTerm(cols, rows)
	errCh := make(chan error, 1)
	go func() {
		errCh <- player.Run(context.Background(), opts, term, newClock())
	}()
	select {
	case err := <-errCh:
		if !errors.Is(err, want) {
			t.Fatalf("err = %v, want %v", err, want)
		}
	case frame := <-term.frames:
		t.Fatalf("drew a frame: %#v", frame)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the run to stop")
	}
}

func firstFrame(t *testing.T, args []string, cols, rows int) player.Frame {
	t.Helper()
	s := startRun(t, args, cols, rows)
	frame := s.frame(t)
	s.cancel()
	s.wait(t)
	return frame
}

type runSession struct {
	term   *memTerm
	clock  *memClock
	cancel context.CancelFunc
	errCh  chan error
}

func startRun(t *testing.T, args []string, cols, rows int) *runSession {
	t.Helper()
	opts, err := player.ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs(%q): %v", args, err)
	}
	term := newTerm(cols, rows)
	clock := newClock()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- player.Run(ctx, opts, term, clock)
	}()
	return &runSession{term: term, clock: clock, cancel: cancel, errCh: errCh}
}

func (s *runSession) key(t *testing.T, k player.Key) {
	t.Helper()
	select {
	case s.term.keys <- k:
	case err := <-s.errCh:
		t.Fatalf("run ended while sending %q: %v", string(k), err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out sending a key")
	}
}

func (s *runSession) frame(t *testing.T) player.Frame {
	t.Helper()
	select {
	case frame := <-s.term.frames:
		return frame
	case err := <-s.errCh:
		t.Fatalf("run ended before a frame: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a frame")
	}
	return player.Frame{}
}

func (s *runSession) fireAbandoned(t *testing.T) {
	t.Helper()
	done := make(chan bool, 1)
	go func() {
		done <- s.clock.FireAbandoned()
	}()
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("no abandoned tick")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("abandoned tick was not received")
	}
}

func (s *runSession) wait(t *testing.T) error {
	t.Helper()
	select {
	case err := <-s.errCh:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the run to stop")
	}
	return nil
}

type memTerm struct {
	mu                  sync.Mutex
	cols, rows          int
	sizeErr, drawErr    error
	frames              chan player.Frame
	keys                chan player.Key
	sizeCalls, keyCalls int
	events              []string
}

func newTerm(cols, rows int) *memTerm {
	return &memTerm{
		cols:   cols,
		rows:   rows,
		frames: make(chan player.Frame),
		keys:   make(chan player.Key),
	}
}

func (m *memTerm) Size() (int, int, error) {
	m.mu.Lock()
	m.sizeCalls++
	m.events = append(m.events, "size")
	err := m.sizeErr
	cols, rows := m.cols, m.rows
	m.mu.Unlock()
	if err != nil {
		return 0, 0, err
	}
	return cols, rows, nil
}

func (m *memTerm) Draw(f player.Frame) error {
	m.mu.Lock()
	m.events = append(m.events, "draw")
	err := m.drawErr
	m.mu.Unlock()
	if err != nil {
		return err
	}
	m.frames <- f
	return nil
}

func (m *memTerm) Keys() <-chan player.Key {
	m.mu.Lock()
	m.keyCalls++
	m.events = append(m.events, "keys")
	ch := m.keys
	m.mu.Unlock()
	return ch
}

func (m *memTerm) snapshot() (sizeCalls, keyCalls int, events []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sizeCalls, m.keyCalls, append([]string(nil), m.events...)
}

type memClock struct {
	mu        sync.Mutex
	delay     time.Duration
	calls     int
	pending   chan time.Time
	abandoned []chan time.Time
}

func newClock() *memClock { return &memClock{} }

func (c *memClock) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time)
	c.mu.Lock()
	if c.pending != nil {
		c.abandoned = append(c.abandoned, c.pending)
	}
	c.pending = ch
	c.delay = d
	c.calls++
	c.mu.Unlock()
	return ch
}

func (c *memClock) Delay() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.delay
}

func (c *memClock) Calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func (c *memClock) Fire() bool {
	c.mu.Lock()
	ch := c.pending
	c.pending = nil
	c.mu.Unlock()
	if ch == nil {
		return false
	}
	ch <- time.Now()
	return true
}

func (c *memClock) FireAbandoned() bool {
	c.mu.Lock()
	if len(c.abandoned) == 0 {
		c.mu.Unlock()
		return false
	}
	ch := c.abandoned[0]
	c.abandoned = c.abandoned[1:]
	c.mu.Unlock()
	ch <- time.Now()
	return true
}

func sameRows(a, b player.Frame) bool {
	if len(a.Rows) != len(b.Rows) {
		return false
	}
	for i := range a.Rows {
		if a.Rows[i] != b.Rows[i] {
			return false
		}
	}
	return true
}

func framesEqual(a, b player.Frame) bool {
	if a.Status != b.Status || len(a.Rows) != len(b.Rows) {
		return false
	}
	for i := range a.Rows {
		if a.Rows[i] != b.Rows[i] {
			return false
		}
	}
	return true
}

func gridFromFrame(t *testing.T, frame player.Frame) [][]bool {
	t.Helper()
	grid := make([][]bool, len(frame.Rows))
	for r, row := range frame.Rows {
		runes := []rune(row)
		if len(runes)%2 != 0 {
			t.Fatalf("row %d has odd rune count %d", r, len(runes))
		}
		grid[r] = make([]bool, len(runes)/2)
		for c := 0; c < len(grid[r]); c++ {
			cell := string(runes[c*2 : c*2+2])
			switch cell {
			case "██":
				grid[r][c] = true
			case "  ":
				grid[r][c] = false
			default:
				t.Fatalf("bad cell %q", cell)
			}
		}
	}
	return grid
}

func assertGrid(t *testing.T, got, want [][]bool) {
	t.Helper()
	if gridsEqual(got, want) {
		return
	}
	t.Fatalf("got\n%s\nwant\n%s", formatGrid(got), formatGrid(want))
}

func gridsEqual(a, b [][]bool) bool {
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
	var b strings.Builder
	for r, row := range grid {
		if r > 0 {
			b.WriteByte('\n')
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

func assertCells(t *testing.T, row string) {
	t.Helper()
	runes := []rune(row)
	if len(runes)%2 != 0 {
		t.Fatalf("row %q has an odd rune count", row)
	}
	for i := 0; i < len(runes); i += 2 {
		cell := string(runes[i : i+2])
		if cell != "██" && cell != "  " {
			t.Fatalf("cell %q is not a live or dead glyph", cell)
		}
	}
}
