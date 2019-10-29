package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
)

func main() {
	lex := flag.Bool("x", false, "lex")
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(21)
	}
	defer r.Close()

	if *lex {
		err = scanFile(r)
	} else {
		err = parseFile(r)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(23)
	}
}

func scanFile(r io.Reader) error {
	s, err := Scan(r)
	if err != nil {
		return err
	}
	for tok := s.Scan(); tok.Type != EOF; tok = s.Scan() {
		fmt.Println(tok)
	}
	return nil
}

func parseFile(r io.Reader) error {
	return Parse(r)
}

var keywords = []string{
	"if",
	"then",
	"else",
	"fi",
	"and",
	"or",
	"enum",
	"polynomial",
	"pointpair",
	"block",
	"import",
	"exit",
	"include",
	"data",
	"declare",
	"define",
}

type Parser struct {
	scan *Scanner

	curr Token
	peek Token

	kwords map[string]func() error
}

func Parse(r io.Reader) error {
	scan, err := Scan(r)
	if err != nil {
		return err
	}
	p := Parser{scan: scan}

	p.kwords = map[string]func() error{
		"data":       p.parseData,
		"block":      p.parseBlock,
		"enum":       p.parsePair,
		"pointpair":  p.parsePair,
		"polynomial": p.parsePair,
		"declare":    p.parseDeclare,
		"define":     p.parseDefine,
	}

	p.nextToken()
	p.nextToken()

	return p.Parse()
}

func (p *Parser) Parse() error {
	p.skipToken(Comment)
	if p.curr.Type == Keyword && p.curr.Literal == "import" {
		if err := p.parseImport(); err != nil {
			return err
		}
	}

	var err error
	for !p.isDone() {
		p.skipToken(Newline)
		switch p.curr.Type {
		case Keyword:
			if fn, ok := p.kwords[p.curr.Literal]; !ok {
				err = fmt.Errorf("parse: unknown keyword: %s", p.curr.Literal)
			} else {
				err = fn()
			}
		case Comment:
			p.skipToken(Comment)
		default:
			err = fmt.Errorf("parse: unexpected token: %s", p.curr)
		}
		if err != nil {
			break
		}
	}
	return err
}

func (p *Parser) parseDeclare() error {
	return nil
}

func (p *Parser) parseDefine() error {
	return nil
}

func (p *Parser) parseData() error {
	fmt.Println("parseData", p.curr)
	p.nextToken()
	if p.curr.Type != lparen {
		return fmt.Errorf("parseData: unexpected token %s", p.curr)
	}
	p.nextToken()
	p.skipToken(Newline)
	for p.curr.Type != rparen {
		switch p.curr.Type {
		case Comment:
			p.skipToken(Comment)
		case Keyword:
			if lit := p.curr.Literal; lit == "include" {
				if err := p.parseInclude(); err != nil {
					return err
				}
			} else if lit == "if" {

			} else {
				return fmt.Errorf("parseData: unexpected keyword %s", p.curr)
			}
		case Ident, Text:
			fmt.Println(">> parseData (ident)", p.curr)
			p.nextToken()
			if p.curr.Type == lsquare {
				if err := p.parseProperties(); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("parseData: unexpected token %s", p.curr)
		}
		p.skipToken(Comment)
		p.skipToken(Newline)
	}
	p.nextToken()
	p.skipToken(Newline)
	return nil
}

func (p *Parser) parseInclude() error {
	p.nextToken()
	if !(p.curr.Type == Ident || p.curr.Type == Text) {
		return fmt.Errorf("parseInclude: unexpected token %s", p.curr)
	}
	fmt.Println(">> include", p.curr)
	p.nextToken()
	return nil
}

func (p *Parser) parseProperties() error {
	for p.curr.Type != rsquare {
		p.nextToken()
		if p.curr.Type != Ident {
			return fmt.Errorf("parseProperties: unexpected token %s", p.curr)
		}
		key := p.curr.Literal
		p.nextToken()
		if p.curr.Type != equal {
			return fmt.Errorf("parseProperties: unexpected token %s", p.curr)
		}
		p.nextToken()
		switch p.curr.Type {
		case Ident:
		case Text:
		case Integer:
		default:
			return fmt.Errorf("parseProperties: unexpected token %s", p.curr)
		}
		fmt.Println("> parseProperties:", key, p.curr.Literal)
		p.nextToken()
		switch p.curr.Type {
		case rsquare, comma:
		default:
			return fmt.Errorf("parseProperties: unexpected token %s", p.curr)
		}
	}
	p.nextToken()
	p.skipToken(Newline)
	return nil
}

func (p *Parser) parseImport() error {
	fmt.Println("parseImport:", p.curr)
	p.nextToken()
	p.nextToken()
	for p.curr.Type != rparen {
		switch p.curr.Type {
		case Newline:
		case Ident, Text:
			fmt.Println("import file:", p.curr.Literal)
		default:
			return fmt.Errorf("parseImport: unexpected token %s", p.curr)
		}
		p.nextToken()
	}
	p.nextToken()
	p.skipToken(Newline)
	return nil
}

func (p *Parser) parseBlock() error {
	fmt.Println("parseBlock:", p.curr)
	p.nextToken()
	if typ := p.curr.Type; !(typ == Ident || typ == Text) {
		return fmt.Errorf("parseBlock: unexpected token %s", p.curr)
	}
	fmt.Println("> parseBlock:", p.curr)
	p.nextToken()
	if p.curr.Type != lparen {
		return fmt.Errorf("parseBlock: unexpectToken %s", p.curr)
	}
	p.nextToken()
	p.skipToken(Newline)
	for p.curr.Type != rparen {
		if !(p.curr.Type == Ident || p.curr.Type != Text) {
			return fmt.Errorf("parseBlock: unexpected token %s", p.curr)
		}
		param := p.curr.Literal
		p.nextToken()
		fmt.Println(">> parseBlock", param)
		p.skipToken(Newline)
	}
	p.nextToken()
	p.skipToken(Newline)

	return nil
}

func (p *Parser) parsePair() error {
	fmt.Println("parsePair:", p.curr)
	p.nextToken()
	if typ := p.curr.Type; !(typ == Ident || typ == Text) {
		return fmt.Errorf("parsePair: unexpected token %s", p.curr)
	}
	p.nextToken()
	if p.curr.Type != lparen {
		return fmt.Errorf("parsePair: unexpected token %s", p.curr)
	}
	p.nextToken()
	p.skipToken(Newline)
	for p.curr.Type != rparen {
		key := p.curr.Literal
		p.nextToken()
		if p.curr.Type != equal {
			return fmt.Errorf("parsePair: unexpected token %s", p.curr)
		}
		p.nextToken()
		fmt.Println("> parsePair:", key, p.curr.Literal)
		if p.peek.Type != Newline {
			return fmt.Errorf("parsePair: unexpected token %s", p.curr)
		}
		p.nextToken()
		p.nextToken()
	}
	p.nextToken()
	p.skipToken(Newline)
	return nil
}

func (p *Parser) isDone() bool {
	return p.curr.Type == EOF
}

func (p *Parser) skipToken(typ rune) {
	for p.curr.Type == typ {
		p.nextToken()
	}
}

func (p *Parser) nextToken() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}

type Position struct {
	Line   int
	Column int
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

type Token struct {
	Literal string
	Type    rune
	pos     Position
}

func (t Token) String() string {
	var (
		str string
		lit = t.Literal
	)
	switch t.Type {
	case EOF:
		return "<eof>"
	case Ident:
		str = "ident"
	case Keyword:
		str = "keyword"
	case Text:
		str = "text"
	case Integer:
		str = "integer"
	case Float:
		str = "float"
	case Comment:
		str = "comment"
	case Equal:
		return "<equal>"
	case NotEq:
		return "<notequal>"
	case Lesser:
		return "<lesser>"
	case LessEq:
		return "<lesseq>"
	case Greater:
		return "<greater>"
	case GreatEq:
		return "<greateq>"
	case Newline:
		return "<newline>"
	case Illegal:
		str = "illegal"
	default:
		str = "punct"
		lit = string(t.Type)
	}
	return fmt.Sprintf("<%s(%s)>", str, lit)
}

const (
	EOF rune = -(iota + 1)
	Ident
	Text
	Keyword
	Integer
	Float
	Comment
	Equal
	NotEq
	Lesser
	LessEq
	Greater
	GreatEq
	Newline
	Illegal
)

const (
	space      = ' '
	tab        = '\t'
	lparen     = '('
	rparen     = ')'
	lsquare    = '['
	rsquare    = ']'
	comma      = ','
	colon      = ':'
	equal      = '='
	semicolon  = ';'
	newline    = '\n'
	minus      = '-'
	underscore = '_'
	pound      = '#'
	dot        = '.'
	bang       = '!'
	langle     = '<'
	rangle     = '>'
	quote      = '"'
)

type Scanner struct {
	buffer []byte
	pos    int
	next   int
	char   byte

	line   int
	column int
}

func Scan(r io.Reader) (*Scanner, error) {
	sort.Strings(keywords)

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
		// s.skipNewline()
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
	s.char = s.buffer[s.next]
	s.pos = s.next
	s.next++
}

func (s *Scanner) unreadByte() {
	s.next = s.pos
	s.pos--
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
		case dot, newline, semicolon, comma, rparen, space, tab:
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
		tok.Type = NotEq
		if s.char != equal {
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
	s.unreadByte()

	if pos < s.pos {
		tok.Literal = string(s.buffer[pos:s.pos])
	}
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
	return b == equal || b == bang || b == langle || b == rangle
}

func isComment(b byte) bool {
	return b == pound
}

func isBlank(b byte) bool {
	return b == space || b == tab
}
