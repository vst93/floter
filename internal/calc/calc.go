// Package calc evaluates the arithmetic the launcher's search box accepts:
// a small expression language for the calculator row.
//
// It is deliberately the pocket-calculator set — numbers, + - * / % ^,
// parentheses and unary signs — because that is what the launcher offers
// while the user types, and it must never run code. Everything is a pure
// function so the ranking tests can pin the results without a window.
package calc

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"unicode"
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
func hasArithmetic(text string) bool {
	digit, operator := false, false
	for _, r := range text {
		switch {
		case unicode.IsDigit(r), r == '.':
			digit = true
		case strings.ContainsRune("+-*/%^()", r):
			operator = true
		}
		// A word character next to letters means a name, not a sum.
		if unicode.IsLetter(r) && r != 'e' && r != 'E' {
			return false
		}
	}
	// A bare number is not a calculator row: searching "1" must not print
	// "1". A sign or parentheses is enough intent.
	return digit && operator
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

// parseUnary is a sign, then a power.
func (p *parser) parseUnary() (float64, error) {
	switch p.peek() {
	case '-':
		p.pos++
		v, err := p.parseUnary()
		return -v, err
	case '+':
		p.pos++
		return p.parseUnary()
	default:
		return p.parsePower()
	}
}

// parsePower is right-associative: 2^3^2 is 2^(3^2).
func (p *parser) parsePower() (float64, error) {
	base, err := p.parsePrimary()
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

// parsePrimary is a number, a parenthesized expression, or a constant.
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
	default:
		return 0, errors.New("calc: unexpected " + strconv.QuoteRune(rune(c)))
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
