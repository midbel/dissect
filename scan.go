package dissect

import (
	"bytes"
	"io"
	"io/ioutil"
	"sort"
)

type Scanner struct {
	buffer []byte
	pos    int
	next   int
	char   byte

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
		s.readByte()
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
	case isLetter(s.char):
		s.scanIdent(&tok)
	case isDigit(s.char) || s.char == minus:
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

	s.readByte()

	return tok
}

func (s *Scanner) readByte() {
	if s.next >= len(s.buffer) {
		s.char = 0
		return
	}
	if char := s.buffer[s.pos]; char == newline {
		s.seen = s.column
		s.line++
		s.column = 0
	}
	s.char = s.buffer[s.next]
	s.pos = s.next
	s.next++

	s.column++
}

func (s *Scanner) unreadByte() {
	s.next = s.pos
	s.pos--
	if char := s.buffer[s.pos]; char == newline {
		s.line--
		s.column = s.seen
	} else {
		s.column--
	}
}

func (s *Scanner) peekByte() byte {
	if s.next >= len(s.buffer) {
		return 0
	}
	return s.buffer[s.next]
}

func (s *Scanner) scanNumber(tok *Token) {
	tok.Type = Integer

	var (
		pos    = s.pos
		nodot  bool
		accept func(byte) bool
	)
	if s.char == minus {
		s.readByte()
	}
	if s.char == '0' {
		switch peek := s.peekByte(); peek {
		case 'x', 'X':
			s.readByte()
			s.readByte()

			accept = isHexa
			nodot = true
		case dot, newline, comma, rparen, space, tab:
		default:
			tok.Type = Illegal
			return
		}
	}
	if accept == nil {
		accept = isDigit
	}

	for accept(s.char) {
		s.readByte()
	}
	switch {
	case s.char == dot && !nodot:
		s.readByte()
		for accept(s.char) {
			s.readByte()
		}
		tok.Type = Float
	case s.char == dot && nodot:
		tok.Type = Illegal
		return
	default:
	}

	tok.Literal = string(s.buffer[pos:s.pos])
	s.unreadByte()
}

func (s *Scanner) scanText(tok *Token) {
	s.readByte()

	pos := s.pos
	for s.char != quote {
		s.readByte()
	}
	tok.Type = Text
	tok.Literal = string(s.buffer[pos:s.pos])
}

func (s *Scanner) scanIdent(tok *Token) {
	pos := s.pos
	for isIdent(s.char) && s.char != 0 {
		s.readByte()
	}

	tok.Literal = string(s.buffer[pos:s.pos])
	tok.Type = Ident

	s.unreadByte()

	if tok.Literal == "true" || tok.Literal == "false" {
		tok.Type = Bool
		return
	}

	ix := sort.SearchStrings(keywords, tok.Literal)
	if ix < len(keywords) && keywords[ix] == tok.Literal {
		tok.Type = Keyword
	}
}

func (s *Scanner) scanOperator(tok *Token) {
	switch peek := s.peekByte(); {
	case s.char == equal:
		tok.Type = equal
		if peek == s.char {
			s.readByte()
			tok.Type = Equal
		}
	case s.char == langle:
		tok.Type = Lesser
		if peek == equal {
			s.readByte()
			tok.Type = LessEq
		}
	case s.char == rangle:
		tok.Type = Greater
		if peek == equal {
			s.readByte()
			tok.Type = GreatEq
		}
	case s.char == bang:
		s.readByte()
		if s.char != equal {
			tok.Type = Not
		} else {
			tok.Type = NotEq
		}
	case s.char == ampersand:
		s.readByte()
		tok.Type = And
		if s.char != ampersand {
			tok.Type = Illegal
		}
	case s.char == pipe:
		s.readByte()
		tok.Type = Or
		if s.char != pipe {
			tok.Type = Illegal
		}
	}
}

func (s *Scanner) scanComment(tok *Token) {
	s.readByte()
	s.skipBlank()

	pos := s.pos
	for s.char != newline {
		s.readByte()
	}

	tok.Literal = string(s.buffer[pos:s.pos])
	tok.Type = Comment
}

func (s *Scanner) skipNewline() {
	for s.char == newline {
		s.readByte()
	}
	if s.char != 0 {
		s.unreadByte()
	}
}

func (s *Scanner) skipBlank() {
	for isBlank(s.char) {
		s.readByte()
	}
}

func isIdent(b byte) bool {
	return isLetter(b) || isDigit(b) || b == minus || b == underscore
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isHexa(b byte) bool {
	return isDigit(b) || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func isOp(b byte) bool {
	return b == equal || b == bang || b == langle || b == rangle || b == ampersand || b == pipe
}

func isComment(b byte) bool {
	return b == pound
}

func isBlank(b byte) bool {
	return b == space || b == tab
}
