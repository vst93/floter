// Package calc evaluates the arithmetic the launcher's search box accepts:
// a small expression language for the calculator row.
//
// It is deliberately the pocket-calculator set — numbers, + - * / % ^,
// parentheses, unary signs, postfix factorial, the `PI`/`E` constants and the
// scalar function family the old build got from `expr-eval-fork` (`sqrt`,
// `sin`, `log`, `abs`, `min`, `pow`, …) — because that is what the launcher
// offers while the user types, and it must never run code. Everything is a
// pure function so the ranking tests can pin the results without a window.
package calc

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// ErrNotAnExpression is what Eval returns for input that is not arithmetic
// at all, so the caller can tell "the user is searching" from "the user
// typed a broken sum".
var ErrNotAnExpression = errors.New("calc: not an expression")

// Eval evaluates an arithmetic expression. A leading "=" is allowed, as the
// old calculator's key made the intent explicit. It returns
// ErrNotAnExpression for input that holds no operator or digit, and an
// error for a malformed or unrepresentable sum.
func Eval(expr string) (float64, error) {
	text := strings.TrimSpace(expr)
	text = strings.TrimSpace(strings.TrimPrefix(text, "="))
	if text == "" || !hasArithmetic(text) {
		return 0, ErrNotAnExpression
	}
	p := &parser{src: text}
	v, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	p.skipSpace()
	if p.pos < len(p.src) {
		return 0, errors.New("calc: unexpected " + strconv.QuoteRune(rune(p.src[p.pos])))
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, errors.New("calc: the result is not a number")
	}
	return v, nil
}

// The result-printing policy, copied from the old build's `calculator.ts`
// (`CALCULATOR_PRECISION` / `CALCULATOR_SCIENTIFIC_HIGH` / `_LOW`): twelve
// significant digits, and scientific notation outside the readable band.
const (
	calculatorPrecision      = 12
	calculatorScientificHigh = 1e15
	calculatorScientificLow  = 1e-9
)

// Format renders a value the way the launcher shows it: the old build's
// `formatCalculatorResult` (src/calculator.ts:87-96), policy for policy.
//
//   - a non-finite value has no printed form and is empty (the caller refuses
//     it before this point);
//   - a magnitude at or above 1e15, or below 1e-9, prints in scientific
//     notation with twelve significant digits and its trailing zeros trimmed
//     (`1e+20`, `9.9e-10`);
//   - otherwise the value is rounded to twelve significant digits and printed
//     the way JavaScript's `String(number)` would — decimal while the decimal
//     exponent stays in [-6, 20], scientific outside it (`1e-7`, but
//     `0.000001`).
//
// The old build reached that last spelling through `Number(x.toPrecision(12))`
// and JS's own number-to-string; this reproduces both halves so `0.1+0.2` is
// `0.3` and `1/3` is `0.333333333333` (twelve digits, not the ten a `%g`
// default would print).
func Format(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return ""
	}
	magnitude := math.Abs(v)
	if magnitude != 0 && (magnitude >= calculatorScientificHigh || magnitude < calculatorScientificLow) {
		return trimExponential(strconv.FormatFloat(v, 'e', calculatorPrecision-1, 64))
	}
	return jsNumberString(roundSignificant(v, calculatorPrecision))
}

// roundSignificant rounds v to n significant digits: the
// `Number(x.toPrecision(n))` half of the old build's policy. The exponential
// spelling carries the rounded digits, and parsing it back gives the double the
// old build then printed.
func roundSignificant(v float64, n int) float64 {
	rounded, err := strconv.ParseFloat(strconv.FormatFloat(v, 'e', n-1, 64), 64)
	if err != nil {
		return v
	}
	return rounded
}

// trimExponential drops the trailing zeros of a scientific mantissa, leaving
// `1e+20` from `1.00000000000e+20` (the old build's `trimExponential`).
func trimExponential(text string) string {
	i := strings.IndexByte(text, 'e')
	if i < 0 {
		return text
	}
	mantissa := strings.TrimRight(text[:i], "0")
	mantissa = strings.TrimSuffix(mantissa, ".")
	return normalizeExponent(mantissa + text[i:])
}

// jsNumberString prints a double the way JavaScript's `String(number)` does:
// decimal notation while the decimal exponent stays in [-6, 20], scientific
// outside it, with the exponent's leading zeros stripped (`1e-7`, not Go's
// `1e-07`).
func jsNumberString(v float64) string {
	if v == 0 {
		return "0"
	}
	exponent := decimalExponent(v)
	if exponent >= 21 || exponent <= -7 {
		return normalizeExponent(strconv.FormatFloat(v, 'e', -1, 64))
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// decimalExponent is the base-ten exponent of v's leading digit, read off the
// shortest scientific spelling so a value that is exactly a power of ten does
// not depend on floating-point logarithms.
func decimalExponent(v float64) int {
	text := strconv.FormatFloat(v, 'e', -1, 64)
	i := strings.IndexByte(text, 'e')
	if i < 0 {
		return 0
	}
	exponent, err := strconv.Atoi(text[i+1:])
	if err != nil {
		return 0
	}
	return exponent
}

// normalizeExponent strips the leading zeros Go pads an exponent with, so the
// printed form matches the old build's (`1e-7` / `1e+20`).
func normalizeExponent(text string) string {
	i := strings.IndexByte(text, 'e')
	if i < 0 {
		return text
	}
	mantissa, exponent := text[:i], text[i+1:]
	sign := ""
	if len(exponent) > 0 && (exponent[0] == '+' || exponent[0] == '-') {
		sign, exponent = string(exponent[0]), exponent[1:]
	}
	exponent = strings.TrimLeft(exponent, "0")
	if exponent == "" {
		exponent = "0"
	}
	return mantissa + "e" + sign + exponent
}

// hasArithmetic reports whether the text looks like a sum rather than a
// search: it must hold a digit and either an operator or a sign that is not
// part of a name. "1Password" is a search; "1+1", "2^10" and "-3" are sums.
//
// The old build's evaluator had no such gate — it simply refused anything the
// parser could not read — but the launcher needs one so a bare number stays a
// search. It now knows the same vocabulary the parser does: a function call
// (`sqrt(4)`) or a constant (`PI`) is intent on its own, while a name that is
// neither is a search term.
func hasArithmetic(text string) bool {
	digit, operator, named := false, false, false
	for i := 0; i < len(text); {
		switch c := text[i]; {
		case c == '.' || (c >= '0' && c <= '9'):
			digit = true
			i++
		case strings.IndexByte("+-*/%^()!", c) >= 0:
			operator = true
			i++
		case isNameStart(c):
			// A lone `e`/`E` right after a number's digits is that number's
			// exponent (`1e3`), not the constant and not a name.
			if (c == 'e' || c == 'E') && digitBefore(text, i) {
				i++
				continue
			}
			start := i
			for i < len(text) && isNamePart(text[i]) {
				i++
			}
			name := text[start:i]
			switch {
			case isConstantName(name):
				named = true
			case isFunctionName(name) && callFollows(text, i):
				named = true
			default:
				// A word the parser does not know is a search, not a sum.
				return false
			}
		default:
			i++
		}
	}
	// A bare number is not a calculator row: searching "1" must not print
	// "1". A sign, parentheses, a function call or a constant is enough intent.
	return named || (digit && operator)
}

// digitBefore reports whether the byte before i is part of a number, so
// `1e3`'s `e` is read as an exponent and `2*E`'s as the constant.
func digitBefore(text string, i int) bool {
	if i == 0 {
		return false
	}
	c := text[i-1]
	return c == '.' || (c >= '0' && c <= '9')
}

// callFollows reports whether an opening parenthesis is the next non-space
// byte, the mark of a function call rather than a bare word.
func callFollows(text string, i int) bool {
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	return i < len(text) && text[i] == '('
}

func isNameStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isNamePart(c byte) bool {
	return isNameStart(c) || (c >= '0' && c <= '9')
}

func isConstantName(name string) bool {
	_, ok := calcConstants[name]
	return ok
}

func isFunctionName(name string) bool {
	_, ok := calcFunctions[name]
	return ok
}

// calcConstants and calcFunctions are the old build's `expr-eval-fork` default
// vocabulary (src/calculator.ts:100-200): the constants `PI`/`E` and the scalar
// built-ins the hardened parser leaves enabled. `random` is absent because the
// old build deletes it; the array helpers (`map`, `fold`, `filter`, `indexOf`,
// `join`, `sum`) and `if` are absent because this language has no array or
// boolean literals to feed them. `ln`/`lg` are the fork's spellings of
// `log`/`log10`.
var calcConstants = map[string]float64{
	"PI": math.Pi,
	"E":  math.E,
}

// builtin is one named function: its arity (`maxArgs` -1 is variadic) and the
// value it computes. Non-finite results (`sqrt(-1)`, `1/0`) are refused by Eval
// at the end, exactly as the old build's `Number.isFinite` check does.
type builtin struct {
	minArgs int
	maxArgs int
	fn      func(args []float64) float64
}

func unary(fn func(float64) float64) builtin {
	return builtin{1, 1, func(args []float64) float64 { return fn(args[0]) }}
}

var calcFunctions = map[string]builtin{
	// expr-eval's `unaryOps`: one argument, called as `sin(x)`.
	"sin":   unary(math.Sin),
	"cos":   unary(math.Cos),
	"tan":   unary(math.Tan),
	"asin":  unary(math.Asin),
	"acos":  unary(math.Acos),
	"atan":  unary(math.Atan),
	"sinh":  unary(math.Sinh),
	"cosh":  unary(math.Cosh),
	"tanh":  unary(math.Tanh),
	"asinh": unary(math.Asinh),
	"acosh": unary(math.Acosh),
	"atanh": unary(math.Atanh),
	"sqrt":  unary(math.Sqrt),
	"cbrt":  unary(math.Cbrt),
	"log":   unary(math.Log),
	"ln":    unary(math.Log),
	"log2":  unary(math.Log2),
	"lg":    unary(math.Log10),
	"log10": unary(math.Log10),
	"expm1": unary(math.Expm1),
	"log1p": unary(math.Log1p),
	"abs":   unary(math.Abs),
	"ceil":  unary(math.Ceil),
	"floor": unary(math.Floor),
	"round": unary(jsRound),
	"trunc": unary(math.Trunc),
	"exp":   unary(math.Exp),
	"sign":  unary(sign),
	// expr-eval's `functions`: the named family.
	"fac":     unary(func(x float64) float64 { return math.Gamma(x + 1) }),
	"gamma":   unary(math.Gamma),
	"roundTo": {2, 2, func(a []float64) float64 { return roundTo(a[0], a[1]) }},
	"pow":     {2, 2, func(a []float64) float64 { return math.Pow(a[0], a[1]) }},
	"atan2":   {2, 2, func(a []float64) float64 { return math.Atan2(a[0], a[1]) }},
	"hypot":   {2, -1, func(a []float64) float64 { return fold(a, math.Hypot) }},
	"pyt":     {2, -1, func(a []float64) float64 { return fold(a, math.Hypot) }},
	"min":     {1, -1, func(a []float64) float64 { return fold(a, math.Min) }},
	"max":     {1, -1, func(a []float64) float64 { return fold(a, math.Max) }},
}

// fold applies a binary operation left to right, the shape the fork's variadic
// `hypot`/`pyt`/`min`/`max` share.
func fold(args []float64, op func(a, b float64) float64) float64 {
	out := args[0]
	for _, v := range args[1:] {
		out = op(out, v)
	}
	return out
}

// jsRound is JavaScript's `Math.round`: halfway cases go towards +Infinity
// (`Math.round(-2.5)` is -2, where Go's `math.Round` gives -3).
func jsRound(x float64) float64 {
	return math.Floor(x + 0.5)
}

// sign is JavaScript's `Math.sign`, keeping the sign of a zero and passing NaN
// through.
func sign(x float64) float64 {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	default:
		return x
	}
}

// roundTo rounds value to place decimal places, shifting through the decimal
// string the way the fork's `roundTo` does, so 2.345 to two places is 2.35
// rather than the 2.34 a float multiply would give.
func roundTo(value, place float64) float64 {
	if place == 0 {
		return jsRound(value)
	}
	if math.IsNaN(value) || place != math.Trunc(place) {
		return math.NaN()
	}
	return shiftDecimal(jsRound(shiftDecimal(value, int(place))), -int(place))
}

// shiftDecimal returns v × 10^exp by re-spelling v's shortest decimal form, so
// the rounding above carries no multiply of its own.
func shiftDecimal(v float64, exp int) float64 {
	text := strconv.FormatFloat(v, 'e', -1, 64)
	i := strings.IndexByte(text, 'e')
	if i < 0 {
		return v
	}
	base, err := strconv.Atoi(text[i+1:])
	if err != nil {
		return v
	}
	out, err := strconv.ParseFloat(text[:i]+"e"+strconv.Itoa(base+exp), 64)
	if err != nil {
		return v
	}
	return out
}

// parser is a recursive-descent parser over the expression's bytes.
type parser struct {
	src string
	pos int
}

func (p *parser) skipSpace() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t') {
		p.pos++
	}
}

func (p *parser) peek() byte {
	p.skipSpace()
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

// parseExpr is the lowest precedence: addition and subtraction.
func (p *parser) parseExpr() (float64, error) {
	v, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		switch p.peek() {
		case '+':
			p.pos++
			rhs, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			v += rhs
		case '-':
			p.pos++
			rhs, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			v -= rhs
		default:
			return v, nil
		}
	}
}

// parseTerm is multiplication, division and modulo.
func (p *parser) parseTerm() (float64, error) {
	v, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	for {
		switch p.peek() {
		case '*':
			p.pos++
			rhs, err := p.parseUnary()
			if err != nil {
				return 0, err
			}
			v *= rhs
		case '/':
			p.pos++
			rhs, err := p.parseUnary()
			if err != nil {
				return 0, err
			}
			if rhs == 0 {
				return 0, errors.New("calc: division by zero")
			}
			v /= rhs
		case '%':
			p.pos++
			rhs, err := p.parseUnary()
			if err != nil {
				return 0, err
			}
			if rhs == 0 {
				return 0, errors.New("calc: division by zero")
			}
			v = math.Mod(v, rhs)
		default:
			return v, nil
		}
	}
}

// parseUnary is a sign or a factorial, then a power.
func (p *parser) parseUnary() (float64, error) {
	switch p.peek() {
	case '-':
		p.pos++
		v, err := p.parseUnary()
		return -v, err
	case '+':
		p.pos++
		return p.parseUnary()
	case '!':
		// The fork's `!` is usable on either side: `!5` and `5!` are both 120.
		p.pos++
		v, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		return math.Gamma(v + 1), nil
	default:
		return p.parsePower()
	}
}

// parsePower is right-associative: 2^3^2 is 2^(3^2).
func (p *parser) parsePower() (float64, error) {
	base, err := p.parsePostfix()
	if err != nil {
		return 0, err
	}
	if p.peek() != '^' {
		return base, nil
	}
	p.pos++
	exp, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	return math.Pow(base, exp), nil
}

// parsePostfix is the factorial: it binds tighter than a power (`2^3!` is
// 2^6) and tighter than a sign (`-3!` is -(3!)), and it stacks (`3!!`).
func (p *parser) parsePostfix() (float64, error) {
	v, err := p.parsePrimary()
	if err != nil {
		return 0, err
	}
	for p.peek() == '!' {
		p.pos++
		v = math.Gamma(v + 1)
	}
	return v, nil
}

// parsePrimary is a number, a parenthesized expression, a constant, or a
// function call.
func (p *parser) parsePrimary() (float64, error) {
	p.skipSpace()
	if p.pos >= len(p.src) {
		return 0, errors.New("calc: the expression ends early")
	}
	switch c := p.src[p.pos]; {
	case c == '(':
		p.pos++
		v, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, errors.New("calc: a parenthesis is not closed")
		}
		p.pos++
		return v, nil
	case c == ')':
		return 0, errors.New("calc: unexpected )")
	case c == '.' || (c >= '0' && c <= '9'):
		return p.parseNumber()
	case isNameStart(c):
		return p.parseName()
	default:
		return 0, errors.New("calc: unexpected " + strconv.QuoteRune(rune(c)))
	}
}

// parseName reads a constant or a function call. A known name without a
// following parenthesis is an error, not a bare variable: this language has
// no variables.
func (p *parser) parseName() (float64, error) {
	start := p.pos
	for p.pos < len(p.src) && isNamePart(p.src[p.pos]) {
		p.pos++
	}
	name := p.src[start:p.pos]
	if value, ok := calcConstants[name]; ok {
		return value, nil
	}
	fn, ok := calcFunctions[name]
	if !ok {
		return 0, errors.New("calc: unknown name " + strconv.Quote(name))
	}
	if p.peek() != '(' {
		return 0, errors.New("calc: " + name + " needs parentheses")
	}
	p.pos++
	args, err := p.parseArgs()
	if err != nil {
		return 0, err
	}
	if len(args) < fn.minArgs || (fn.maxArgs >= 0 && len(args) > fn.maxArgs) {
		return 0, errors.New("calc: wrong number of arguments for " + name)
	}
	return fn.fn(args), nil
}

// parseArgs reads the comma-separated arguments of a call, the opening
// parenthesis already consumed, and consumes the closing one.
func (p *parser) parseArgs() ([]float64, error) {
	args := make([]float64, 0, 2)
	if p.peek() == ')' {
		p.pos++
		return args, nil
	}
	for {
		v, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, v)
		switch p.peek() {
		case ',':
			p.pos++
		case ')':
			p.pos++
			return args, nil
		default:
			return nil, errors.New("calc: a function call is not closed")
		}
	}
}

// parseNumber reads a decimal number, optionally with an exponent.
func (p *parser) parseNumber() (float64, error) {
	start := p.pos
	for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
		p.pos++
	}
	if p.pos < len(p.src) && p.src[p.pos] == '.' {
		p.pos++
		for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
			p.pos++
		}
	}
	if p.pos < len(p.src) && (p.src[p.pos] == 'e' || p.src[p.pos] == 'E') {
		next := p.pos + 1
		if next < len(p.src) && (p.src[next] == '+' || p.src[next] == '-') {
			next++
		}
		if next < len(p.src) && p.src[next] >= '0' && p.src[next] <= '9' {
			p.pos = next
			for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
				p.pos++
			}
		}
	}
	v, err := strconv.ParseFloat(p.src[start:p.pos], 64)
	if err != nil {
		return 0, errors.New("calc: bad number " + strconv.Quote(p.src[start:p.pos]))
	}
	return v, nil
}
