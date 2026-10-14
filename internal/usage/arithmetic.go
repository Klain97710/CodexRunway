package usage

import (
	"errors"
	"fmt"
	"math"
)

// ErrIntegerOverflow reports that summing counters overflowed int64.
var ErrIntegerOverflow = errors.New("usage: integer overflow")

// ErrInvalidValue reports a counter that is negative or not finite.
var ErrInvalidValue = errors.New("usage: invalid value")

// ArithmeticError names the field whose arithmetic failed.
type ArithmeticError struct {
	Err   error
	Field string
}

func (e *ArithmeticError) Error() string { return fmt.Sprintf("%v: %s", e.Err, e.Field) }
func (e *ArithmeticError) Unwrap() error { return e.Err }

func overflow(field string) error { return &ArithmeticError{Err: ErrIntegerOverflow, Field: field} }
func invalid(field string) error  { return &ArithmeticError{Err: ErrInvalidValue, Field: field} }

func checkedAdd(a, b int64, field string) (int64, error) {
	c := a + b
	if (c > a) == (b > 0) {
		return c, nil
	}
	return 0, overflow(field)
}

func checkedFiniteSum(values []float64, field string) (float64, error) {
	var result float64
	for _, v := range values {
		if math.IsInf(result, 0) || math.IsNaN(result) || math.IsInf(v, 0) || math.IsNaN(v) {
			return 0, invalid(field)
		}
		sum := float64(result + v)
		if math.IsInf(sum, 0) || math.IsNaN(sum) {
			return 0, invalid(field)
		}
		result = sum
	}
	return result, nil
}

func (u TokenUsage) add(o TokenUsage) (TokenUsage, error) {
	in, err := checkedAdd(u.InputTokens, o.InputTokens, "input tokens")
	if err != nil {
		return TokenUsage{}, err
	}
	cached, err := checkedAdd(u.CachedInputTokens, o.CachedInputTokens, "cached input tokens")
	if err != nil {
		return TokenUsage{}, err
	}
	out, err := checkedAdd(u.OutputTokens, o.OutputTokens, "output tokens")
	if err != nil {
		return TokenUsage{}, err
	}
	return TokenUsage{in, cached, out}, nil
}

// Add returns t + o, failing with an ArithmeticError on int64 overflow.
func (t Totals) Add(o Totals) (Totals, error) {
	var r Totals
	var err error
	if r.TotalTokens, err = checkedAdd(t.TotalTokens, o.TotalTokens, "total tokens"); err != nil {
		return Totals{}, err
	}
	if r.UncachedInputTokens, err = checkedAdd(t.UncachedInputTokens, o.UncachedInputTokens, "uncached input tokens"); err != nil {
		return Totals{}, err
	}
	if r.CachedInputTokens, err = checkedAdd(t.CachedInputTokens, o.CachedInputTokens, "cached input tokens"); err != nil {
		return Totals{}, err
	}
	if r.OutputTokens, err = checkedAdd(t.OutputTokens, o.OutputTokens, "output tokens"); err != nil {
		return Totals{}, err
	}
	if r.Turns, err = checkedAdd(t.Turns, o.Turns, "turns"); err != nil {
		return Totals{}, err
	}
	if r.Threads, err = checkedAdd(t.Threads, o.Threads, "threads"); err != nil {
		return Totals{}, err
	}
	return r, nil
}

func sumTotals(values []Totals) (Totals, error) {
	var r Totals
	for _, v := range values {
		var err error
		if r, err = r.Add(v); err != nil {
			return Totals{}, err
		}
	}
	return r, nil
}

// totalsFromUsage validates a token triple and builds Totals.
func totalsFromUsage(u TokenUsage, turns, threads int64) (Totals, error) {
	if u.InputTokens < 0 || u.CachedInputTokens < 0 || u.OutputTokens < 0 ||
		u.CachedInputTokens > u.InputTokens || turns < 0 || threads < 0 {
		return Totals{}, invalid("token usage")
	}
	uncached := u.InputTokens - u.CachedInputTokens
	input, err := checkedAdd(uncached, u.CachedInputTokens, "input tokens")
	if err != nil {
		return Totals{}, err
	}
	total, err := checkedAdd(input, u.OutputTokens, "total tokens")
	if err != nil {
		return Totals{}, err
	}
	return Totals{
		TotalTokens:         total,
		UncachedInputTokens: uncached,
		CachedInputTokens:   u.CachedInputTokens,
		OutputTokens:        u.OutputTokens,
		Turns:               turns,
		Threads:             threads,
	}, nil
}
