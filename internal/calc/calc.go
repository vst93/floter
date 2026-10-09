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

// Format renders a value the way the launcher shows it: whole numbers
// without a decimal point, others with ten significant digits, which is
// enough for a search box and hides float noise (0.1+0.2 -> 0.3).
func Format(v float64) string {
	if v == 0 {
		return "0"
	}
	if math.Trunc(v) == v && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', 10, 64)
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
