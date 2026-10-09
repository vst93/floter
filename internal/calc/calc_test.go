package calc

import (
	"errors"
	"math"
	"testing"
)

func TestEval(t *testing.T) {
	cases := []struct {
		expr string
		want float64
	}{
		{"1+1", 2},
		{"=1+1", 2},
		{" 2 * 3 ", 6},
		{"2+3*4", 14},
		{"(2+3)*4", 20},
		{"10/4", 2.5},
		{"10%3", 1},
		{"2^10", 1024},
		{"2^3^2", 512}, // right-associative
		{"-3+5", 2},
		{"-(2+3)", -5},
		{"--4", 4},
		{"+4", 4},
		{"1.5*2", 3},
		{"0.1+0.2", 0.30000000000000004},
		{"((1+2)*(3+4))", 21},
		{"1e3+1", 1001},
	}
	for _, tc := range cases {
		got, err := Eval(tc.expr)
		if err != nil {
			t.Errorf("Eval(%q): %v", tc.expr, err)
			continue
		}
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("Eval(%q) = %v, want %v", tc.expr, got, tc.want)
		}
	}
}

func TestEvalRejects(t *testing.T) {
	// Not arithmetic at all: the caller shows no calculator row.
	for _, expr := range []string{"", "   ", "abc", "1Password", "term", "apps", "safari", "="} {
		if _, err := Eval(expr); !errors.Is(err, ErrNotAnExpression) {
			t.Errorf("Eval(%q) = %v, want ErrNotAnExpression", expr, err)
		}
	}
	// Arithmetic intent, but broken or unrepresentable: an error, not
	// ErrNotAnExpression.
	for _, expr := range []string{"1+", "*2", "(1", "1)", "1++", "1/0", "5%0", "2+*3", "1 2 3 +"} {
		_, err := Eval(expr)
		if err == nil {
			t.Errorf("Eval(%q) accepted a broken sum", expr)
		} else if errors.Is(err, ErrNotAnExpression) {
			t.Errorf("Eval(%q) = ErrNotAnExpression, want a parse error", expr)
		}
	}
}

func TestFormat(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{2, "2"},
		{-5, "-5"},
		{1024, "1024"},
		{2.5, "2.5"},
		{1.0 / 3.0, "0.3333333333"},
		{0.1 + 0.2, "0.3"},
	}
	for _, tc := range cases {
		if got := Format(tc.in); got != tc.want {
			t.Errorf("Format(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNotAnExpressionForNamesWithSigns(t *testing.T) {
	// A sign next to a letter is a name, not a sum: the launcher must not
	// print a calculator row while the user searches for it.
	if _, err := Eval("C++"); !errors.Is(err, ErrNotAnExpression) {
		t.Errorf("Eval(C++) = %v, want ErrNotAnExpression", err)
	}
}
