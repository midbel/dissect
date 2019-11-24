package dissect

import (
	"bytes"
	"io"
	"io/ioutil"
	"sort"
	"unicode/utf8"
)

type Scanner struct {
	buffer []byte
	pos    int
	next   int
	char   rune

	line   int
	column int
	seen   int
}

func Scan(r io.Reader) (*Scanner, error) {
	var s Scanner
	if err := s.Reset(r); err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *Scanner) Reset(r io.Reader) error {
	buf, err := ioutil.ReadAll(r)
	if err == nil {
		s.buffer = bytes.ReplaceAll(buf, []byte("\r\n"), []byte("\n"))
		s.line = 1
		s.column = 0
		s.readRune()
	}
	return err
}

func (s *Scanner) Scan() Token {
	var tok Token
	if s.char == 0 {
		tok.Type = EOF
		return tok
	}

	s.skipBlank()

	tok.pos = Position{
		Line:   s.line,
		Column: s.column,
	}

	switch {
	case s.char == dollar:
		s.readRune()
		if tok = s.Scan(); tok.Type != Ident {
			tok.Type = Illegal
		} else {
			tok.Type = Internal
			s.unreadRune()
		}
	case isLetter(s.char) || (s.char == underscore && isLetter(s.peekRune())):
		s.scanIdent(&tok)
	case isDigit(s.char): // || s.char == minus:
		s.scanNumber(&tok)
	case isComment(s.char):
		s.scanComment(&tok)
	case isOp(s.char):
		s.scanOperator(&tok)
	case s.char == quote:
		s.scanText(&tok)
	case s.char == newline:
		tok.Type = Newline
	default:
		tok.Type = rune(s.char)
	}

	s.readRune()

	return tok
}

func (s *Scanner) readRune() {
	if s.next >= len(s.buffer) {
		s.char = EOF
		return
	}
	r, n := utf8.DecodeRune(s.buffer[s.next:])
	if r == utf8.RuneError {
		if n == 0 {
			s.char = EOF
		} else {
			s.char = Illegal
		}
		s.next = len(s.buffer)
	}
	s.char, s.pos, s.next = r, s.next, s.next+n
	if s.char == newline {
		s.line++
		s.seen, s.column = s.column, 0
	} else {
		s.column++
	}
}

func (s *Scanner) unreadRune() {
	if s.next <= 0 || s.char == 0 {
		return
	}

	if s.char == newline {
		s.line--
		s.column = s.seen
	} else {
		s.column--
	}

	s.next, s.pos = s.pos, s.pos-utf8.RuneLen(s.char)
	s.char, _ = utf8.DecodeRune(s.buffer[s.pos:])
}

func (s *Scanner) peekRune() rune {
	if s.next >= len(s.buffer) {
		return EOF
	}
	r, n := utf8.DecodeRune(s.buffer[s.next:])
	if r == utf8.RuneError {
		if n == 0 {
			r = EOF
		} else {
			r = Illegal
		}
	}
	return r
}

func (s *Scanner) scanNumber(tok *Token) {
	tok.Type = Integer

	var (
		pos    = s.pos
		nodot  bool
		accept func(rune) bool
	)

	if s.char == '0' {
		switch peek := s.peekRune(); peek {
		case 'x', 'X':
			s.readRune()
			s.readRune()

			accept = isHexa
			nodot = true
		case dot, newline, comma, rsquare, rparen, space, tab:
		default:
			tok.Type = Illegal
			return
		}
	}
	if accept == nil {
		accept = isDigit
	}

	for accept(s.char) {
		s.readRune()
	}
	switch {
	case s.char == dot && !nodot:
		s.readRune()
		for accept(s.char) {
			s.readRune()
		}
		if s.char == 'e' || s.char == 'E' {
			s.scanExponent()
		}
		tok.Type = Float
	case (s.char == 'e' || s.char == 'E') && !nodot:
		s.scanExponent()
		tok.Type = Float
	case s.char == dot && nodot:
		tok.Type = Illegal
		return
	default:
	}

	tok.Literal = string(s.buffer[pos:s.pos])
	s.unreadRune()
}

func (s *Scanner) scanExponent() {
	s.readRune()
	if s.char == minus {
		s.readRune()
	}
	for isDigit(s.char) {
		s.readRune()
	}
}

func (s *Scanner) scanText(tok *Token) {
	s.readRune()

	pos := s.pos
	for s.char != quote {
		s.readRune()
	}
	tok.Type = Text
	tok.Literal = string(s.buffer[pos:s.pos])
}

func (s *Scanner) scanIdent(tok *Token) {
	pos := s.pos
	for isIdent(s.char) && s.char != 0 {
		s.readRune()
	}

	tok.Literal = string(s.buffer[pos:s.pos])
	tok.Type = Ident

	s.unreadRune()

	if tok.Literal == kwTrue || tok.Literal == kwFalse {
		tok.Type = Bool
		return
	}

	ix := sort.SearchStrings(keywords, tok.Literal)
	if ix < len(keywords) && keywords[ix] == tok.Literal {
		tok.Type = Keyword
	}
}

func (s *Scanner) scanOperator(tok *Token) {
	switch peek := s.peekRune(); {
	case s.char == add:
		tok.Type = Add
	case s.char == mul:
		tok.Type = Mul
	case s.char == div:
		tok.Type = Div
	case s.char == minus:
		tok.Type = Min
	case s.char == equal:
		tok.Type = Assign
		if peek == s.char {
			s.readRune()
			tok.Type = Equal
		}
	case s.char == langle:
		tok.Type = Lesser
		if peek == equal {
			s.readRune()
			tok.Type = LessEq
		}
		if peek == langle {
			s.readRune()
			tok.Type = ShiftLeft
		}
	case s.char == rangle:
		tok.Type = Greater
		if peek == equal {
			s.readRune()
			tok.Type = GreatEq
		}
		if peek == rangle {
			s.readRune()
			tok.Type = ShiftRight
		}
	case s.char == bang:
		if peek := s.peekRune(); peek != equal {
			tok.Type = Not
		} else {
			tok.Type = NotEq
			s.readRune()
		}
	case s.char == ampersand:
		s.readRune()
		tok.Type = And
		if s.char != ampersand {
			tok.Type = BitAnd
		}
	case s.char == pipe:
		s.readRune()
		tok.Type = Or
		if s.char != pipe {
			tok.Type = BitOr
		}
	case s.char == question:
		tok.Type = Cond
	}
}

func (s *Scanner) scanComment(tok *Token) {
	s.readRune()
	s.skipBlank()

	pos := s.pos
	for s.char != newline {
		s.readRune()
	}

	tok.Literal = string(s.buffer[pos:s.pos])
	tok.Type = Comment
}

func (s *Scanner) skipNewline() {
	for s.char == newline {
		s.readRune()
	}
	if s.char != 0 {
		s.unreadRune()
	}
}

func (s *Scanner) skipBlank() {
	for isBlank(s.char) {
		s.readRune()
	}
}

func isIdent(b rune) bool {
	return isLetter(b) || isDigit(b) || b == underscore
}

func isLetter(b rune) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isDigit(b rune) bool {
	return b >= '0' && b <= '9'
}

func isHexa(b rune) bool {
	return isDigit(b) || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func isOp(b rune) bool {
	return b == equal || b == bang || b == langle || b == rangle || b == ampersand || b == pipe || b == add || b == div || b == mul || b == minus || b == question
}

func isComment(b rune) bool {
	return b == pound
}

func isBlank(b rune) bool {
	return b == space || b == tab
}
