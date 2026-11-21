package bybit

import "bytes"

// wsScanner reads the compact JSON that the public stream sends. It accepts only a strict subset of JSON:
// no whitespace outside strings, no escapes, printable ASCII in strings, unsigned integers of at most 18 digits.
// Whatever it refuses is decoded by encoding/json, so a refusal never changes a result.
type wsScanner struct {
	data []byte
	pos  int
}

func (s *wsScanner) consume(c byte) bool {
	if s.pos < len(s.data) && s.data[s.pos] == c {
		s.pos++
		return true
	}

	return false
}

func (s *wsScanner) atEnd() bool {
	return s.pos == len(s.data)
}

func (s *wsScanner) plainString() ([]byte, bool) {
	start, end, ok := s.plainStringSpan()
	if !ok {
		return nil, false
	}

	return s.data[start:end], true
}

func (s *wsScanner) plainStringSpan() (start, end int, ok bool) {
	if !s.consume('"') {
		return 0, 0, false
	}

	start = s.pos
	for i := start; i < len(s.data); i++ {
		c := s.data[i]
		if c == '"' {
			s.pos = i + 1
			return start, i, true
		}

		if c == '\\' || c < 0x20 || c > 0x7e {
			return 0, 0, false
		}
	}

	return 0, 0, false
}

const maxUintLiteralDigits = 18

func (s *wsScanner) uintLiteral() (int64, bool) {
	start := s.pos

	var v int64
	for s.pos < len(s.data) {
		c := s.data[s.pos]
		if c < '0' || c > '9' {
			break
		}

		v = v*10 + int64(c-'0')
		s.pos++

		if s.pos-start > maxUintLiteralDigits {
			return 0, false
		}
	}

	n := s.pos - start
	if n == 0 || (n > 1 && s.data[start] == '0') {
		return 0, false
	}

	return v, true
}

func (s *wsScanner) literal(word string) bool {
	if len(s.data)-s.pos < len(word) || string(s.data[s.pos:s.pos+len(word)]) != word {
		return false
	}

	s.pos += len(word)

	return true
}

func (s *wsScanner) plainScalar() bool {
	if s.pos >= len(s.data) {
		return false
	}

	switch c := s.data[s.pos]; {
	case c == '"':
		_, ok := s.plainString()
		return ok
	case c >= '0' && c <= '9':
		_, ok := s.uintLiteral()
		return ok
	case c == 't':
		return s.literal("true")
	case c == 'f':
		return s.literal("false")
	case c == 'n':
		return s.literal("null")
	}

	return false
}

// skipValue passes one value without validating it: the frame is validated by the decoding that follows.
func (s *wsScanner) skipValue() bool {
	if s.pos >= len(s.data) {
		return false
	}

	switch s.data[s.pos] {
	case '"':
		return s.skipString()
	case '{', '[':
		return s.skipContainer()
	}

	start := s.pos

	var guard numberGuard

	for s.pos < len(s.data) {
		switch s.data[s.pos] {
		case ',', '}', ']':
			return s.pos > start
		}

		if !guard.accept(s.data[s.pos]) {
			return false
		}

		s.pos++
	}

	return false
}

const maxSkippedDigits = 300

// numberGuard refuses the numbers that encoding/json may fail to fit into a float64 when it decodes the frame into
// a map: those with an exponent or with a very long run of digits. It is fed every byte outside strings.
type numberGuard struct {
	run  int
	prev byte
}

func (g *numberGuard) accept(c byte) bool {
	isDigit := c >= '0' && c <= '9'

	if (c == 'e' || c == 'E') && ((g.prev >= '0' && g.prev <= '9') || g.prev == '.') {
		return false
	}

	if isDigit {
		g.run++
	} else {
		g.run = 0
	}

	g.prev = c

	return g.run <= maxSkippedDigits
}

func (s *wsScanner) skipString() bool {
	s.pos++
	for s.pos < len(s.data) {
		switch s.data[s.pos] {
		case '\\':
			s.pos += 2
			continue
		case '"':
			s.pos++
			return true
		}

		s.pos++
	}

	return false
}

func (s *wsScanner) skipContainer() bool {
	var guard numberGuard

	depth := 0

	for s.pos < len(s.data) {
		switch s.data[s.pos] {
		case '"':
			if !s.skipString() {
				return false
			}

			guard = numberGuard{}

			continue
		case '{', '[':
			depth++
			guard = numberGuard{}
		case '}', ']':
			depth--
			if depth == 0 {
				s.pos++
				return true
			}

			guard = numberGuard{}
		default:
			if !guard.accept(s.data[s.pos]) {
				return false
			}
		}

		s.pos++
	}

	return false
}

var publicTopicsByPriority = []struct {
	name  []byte
	topic V5WebsocketPublicTopic
}{
	{name: []byte(V5WebsocketPublicTopicOrderBook), topic: V5WebsocketPublicTopicOrderBook},
	{name: []byte(V5WebsocketPublicTopicKline), topic: V5WebsocketPublicTopicKline},
	{name: []byte(V5WebsocketPublicTopicTicker), topic: V5WebsocketPublicTopicTicker},
	{name: []byte(V5WebsocketPublicTopicTrade), topic: V5WebsocketPublicTopicTrade},
	{name: []byte(V5WebsocketPublicTopicLiquidation), topic: V5WebsocketPublicTopicLiquidation},
}

// scanTopic answers true only for a recognised topic of a compact frame. A repeated top-level "topic" key is
// resolved as a map would resolve it: the last one wins.
func scanTopic(message []byte) (V5WebsocketPublicTopic, bool) {
	s := wsScanner{data: message}
	if !s.consume('{') {
		return "", false
	}

	var topic []byte

	found := false

	for {
		key, ok := s.plainString()
		if !ok || !s.consume(':') {
			return "", false
		}

		if string(key) == "topic" {
			topic, ok = s.plainString()
			if !ok {
				return "", false
			}

			found = true
		} else if !s.skipValue() {
			return "", false
		}

		if s.consume(',') {
			continue
		}

		if s.consume('}') && s.atEnd() {
			break
		}

		return "", false
	}

	if !found {
		return "", false
	}

	for _, t := range publicTopicsByPriority {
		if bytes.Contains(topic, t.name) {
			return t.topic, true
		}
	}

	return "", false
}
