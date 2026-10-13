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

// Format is the old build's `formatCalculatorResult` (src/calculator.ts:87-96)
// policy for policy: twelve significant digits, scientific notation outside the
// readable band, and JS's own `String(number)` spelling inside it. The table
// pins every branch of that policy — including the boundaries where a change of
// one digit flips the spelling.
func TestFormat(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{-0.0, "0"},
		{2, "2"},
		{-5, "-5"},
		{1024, "1024"},
		{2.5, "2.5"},
		// Twelve significant digits, not the ten a `%g` default prints.
		{1.0 / 3.0, "0.333333333333"},
		{0.1 + 0.2, "0.3"},
		// JS `String(number)` stays decimal down to 1e-6 and switches at 1e-7.
		{1e-5, "0.00001"},
		{1e-6, "0.000001"},
		{1e-7, "1e-7"},
		// At 1e-9 the scientific band has not opened yet; below it, it has.
		{1e-9, "1e-9"},
		{9.9e-10, "9.9e-10"},
		// The high band opens at 1e15 and trims its mantissa's trailing zeros.
		{1e15, "1e+15"},
		{1e20, "1e+20"},
		{1.23456789012345e20, "1.23456789012e+20"},
		// A 15-digit integer rounds to twelve digits and prints the rounded value.
		{999999999999999, "1000000000000000"},
		// A non-finite value has no printed form.
		{math.NaN(), ""},
		{math.Inf(1), ""},
		{math.Inf(-1), ""},
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

// TestEvalBuiltins is the old build's `expr-eval-fork` vocabulary
// (src/calculator.ts:100-200): the constants `PI`/`E`, the scalar function
// family, and the postfix factorial. Each row is one call the fork accepts.
func TestEvalBuiltins(t *testing.T) {
	cases := []struct {
		expr string
		want float64
	}{
		// The trig / inverse-trig / hyperbolic family.
		{"sin(0)", 0},
		{"cos(0)", 1},
		{"tan(0)", 0},
		{"asin(0)", 0},
		{"acos(1)", 0},
		{"atan(0)", 0},
		{"sinh(0)", 0},
		{"cosh(0)", 1},
		{"tanh(0)", 0},
		{"asinh(0)", 0},
		{"acosh(1)", 0},
		{"atanh(0)", 0},
		// Roots, logs and exponentials. `ln`/`lg` are the fork's spellings of
		// `log`/`log10`.
		{"sqrt(16)", 4},
		{"cbrt(27)", 3},
		{"log(1)", 0},
		{"ln(E)", 1},
		{"log2(8)", 3},
		{"lg(100)", 2},
		{"log10(100)", 2},
		{"exp(0)", 1},
		{"expm1(0)", 0},
		{"log1p(0)", 0},
		// Rounding and sign. `round` follows JavaScript, so -2.5 is -2.
		{"abs(-3)", 3},
		{"ceil(1.2)", 2},
		{"floor(1.8)", 1},
		{"round(2.5)", 3},
		{"round(-2.5)", -2},
		{"trunc(2.7)", 2},
		{"sign(-3)", -1},
		{"sign(0)", 0},
		// The named family.
		{"fac(5)", 120},
		{"gamma(5)", 24},
		{"roundTo(2.345,2)", 2.35},
		{"pow(2,10)", 1024},
		{"atan2(1,1)", math.Pi / 4},
		{"hypot(3,4)", 5},
		{"pyt(3,4)", 5},
		{"min(1,2,3)", 1},
		{"max(1,2,3)", 3},
		// Constants, alone and in a sum.
		{"PI", math.Pi},
		{"E", math.E},
		{"sin(PI/2)", 1},
		{"2*E", 2 * math.E},
		// The factorial, on either side, stacking, and binding tighter than a
		// power and a sign (`2^3!` is 2^6, `-3!` is -(3!)).
		{"5!", 120},
		{"!5", 120},
		{"3!!", 720},
		{"2^3!", 64},
		{"-3!", -6},
		{"5!+1", 121},
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

// TestEvalFunctionTableMatchesTheFork pins the vocabulary by name: every
// function the fork's default table has (minus `random`, which the old build
// deletes, and the array/conditional helpers this language cannot feed) is
// here, and nothing else is. A rename or a quiet deletion turns red.
func TestEvalFunctionTableMatchesTheFork(t *testing.T) {
	want := []string{
		// unaryOps
		"sin", "cos", "tan", "asin", "acos", "atan",
		"sinh", "cosh", "tanh", "asinh", "acosh", "atanh",
		"sqrt", "cbrt", "log", "log2", "ln", "lg", "log10",
		"expm1", "log1p", "abs", "ceil", "floor", "round", "trunc",
		"exp", "sign",
		// functions
		"fac", "gamma", "roundTo", "pow", "atan2", "hypot", "pyt", "min", "max",
	}
	for _, name := range want {
		if !isFunctionName(name) {
			t.Errorf("calcFunctions is missing %q", name)
		}
	}
	if len(calcFunctions) != len(want) {
		t.Errorf("calcFunctions has %d names, want %d", len(calcFunctions), len(want))
	}
	for _, name := range []string{"random", "map", "fold", "filter", "indexOf", "join", "sum", "if"} {
		if isFunctionName(name) {
			t.Errorf("calcFunctions must not carry %q", name)
		}
	}
	if !isConstantName("PI") || !isConstantName("E") {
		t.Errorf("calcConstants must hold PI and E")
	}
}

// TestEvalRejectsUnknownNames keeps the gate: a name the parser does not know
// is a search, not a sum, so the launcher never prints a row for it.
func TestEvalRejectsUnknownNames(t *testing.T) {
	for _, expr := range []string{"foo(4)", "sign", "sqrt", "1Password", "Safari"} {
		if _, err := Eval(expr); !errors.Is(err, ErrNotAnExpression) {
			t.Errorf("Eval(%q) = %v, want ErrNotAnExpression", expr, err)
		}
	}
	// Arithmetic intent, but a broken call: an error, not ErrNotAnExpression.
	for _, expr := range []string{"sqrt(4", "sqrt()", "pow(2)", "min()", "roundTo(2.345)"} {
		_, err := Eval(expr)
		if err == nil {
			t.Errorf("Eval(%q) accepted a broken call", expr)
		} else if errors.Is(err, ErrNotAnExpression) {
			t.Errorf("Eval(%q) = ErrNotAnExpression, want a parse error", expr)
		}
	}
}
