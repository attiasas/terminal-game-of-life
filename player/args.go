package player

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
)

const usageText = "usage: life [--seed int] [--density 0.30] [--size WxH]"

var sizePattern = regexp.MustCompile(`^([1-9][0-9]*)x([1-9][0-9]*)$`)

// ParseArgs validates args and builds Options. It reads args and nothing else.
func ParseArgs(args []string) (Options, error) {
	fs := flag.NewFlagSet("life", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		seed    int64
		seedSet bool
	)
	fs.Func("seed", "PRNG seed", func(s string) error {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid seed %q", s)
		}
		seed = v
		seedSet = true
		return nil
	})
	density := fs.Float64("density", 0.30, "fraction of live cells")
	size := fs.String("size", "", "grid size WxH")
	if err := fs.Parse(args); err != nil {
		msg := err.Error()
		if errors.Is(err, flag.ErrHelp) {
			msg = usageText
		}
		return Options{}, &UsageError{Msg: msg}
	}
	if fs.NArg() > 0 {
		return Options{}, &UsageError{Msg: fmt.Sprintf("unexpected argument %q\n%s", fs.Arg(0), usageText)}
	}

	if math.IsNaN(*density) || math.IsInf(*density, 0) || *density < 0 || *density > 1 {
		return Options{}, &UsageError{Msg: fmt.Sprintf("density %v is outside [0, 1]\n%s", *density, usageText)}
	}
	if !seedSet {
		var buf [8]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return Options{}, err
		}
		seed = int64(binary.LittleEndian.Uint64(buf[:]))
	}

	opts := Options{ok: true, seed: seed, density: *density}
	if *size == "" {
		return opts, nil
	}
	match := sizePattern.FindStringSubmatch(*size)
	if match == nil {
		return Options{}, &UsageError{Msg: fmt.Sprintf("invalid size %q\n%s", *size, usageText)}
	}
	width, err := strconv.Atoi(match[1])
	if err != nil {
		return Options{}, &UsageError{Msg: fmt.Sprintf("invalid size %q\n%s", *size, usageText)}
	}
	height, err := strconv.Atoi(match[2])
	if err != nil {
		return Options{}, &UsageError{Msg: fmt.Sprintf("invalid size %q\n%s", *size, usageText)}
	}
	opts.width = width
	opts.height = height
	opts.sized = true
	return opts, nil
}
