package bybit

const (
	orderBookKeyTopic = 1 << iota
	orderBookKeyType
	orderBookKeyTS
	orderBookKeyData
)

const (
	orderBookDataKeySymbol = 1 << iota
	orderBookDataKeyBids
	orderBookDataKeyAsks
	orderBookDataKeyUpdateID
	orderBookDataKeySeq
)

var (
	orderBookTopKeys  = []string{"topic", "type", "ts", "data"}
	orderBookDataKeys = []string{"s", "b", "a", "u", "seq"}
)

// orderBookDecoder decodes a compact order book frame. All strings of the result share one copy of the frame,
// made on the first string.
type orderBookDecoder struct {
	wsScanner
	text string
}

func (d *orderBookDecoder) str(start, end int) string {
	if d.text == "" {
		d.text = string(d.data)
	}

	return d.text[start:end]
}

func (d *orderBookDecoder) stringValue() (string, bool) {
	start, end, ok := d.plainStringSpan()
	if !ok {
		return "", false
	}

	return d.str(start, end), true
}

func (d *orderBookDecoder) intValue() (int, bool) {
	v, ok := d.uintLiteral()
	if !ok || int64(int(v)) != v {
		return 0, false
	}

	return int(v), true
}

// decodeOrderBookFast leaves resp untouched when it refuses the frame. It refuses whatever encoding/json could
// decode differently from a plain field by field reading: a repeated known key, a key that matches a known key
// only without regard to case, an unknown key with a non-scalar value, an empty data object.
func decodeOrderBookFast(message []byte, resp *V5WebsocketPublicOrderBookResponse) bool {
	d := orderBookDecoder{wsScanner: wsScanner{data: message}}

	var out V5WebsocketPublicOrderBookResponse

	if !d.consume('{') {
		return false
	}

	seen := 0

	for {
		key, ok := d.plainString()
		if !ok || !d.consume(':') {
			return false
		}

		bit := 0

		switch string(key) {
		case "topic":
			bit = orderBookKeyTopic
			out.Topic, ok = d.stringValue()
		case "type":
			bit = orderBookKeyType
			out.Type, ok = d.stringValue()
		case "ts":
			bit = orderBookKeyTS
			out.TimeStamp, ok = d.uintLiteral()
		case "data":
			bit = orderBookKeyData
			ok = d.orderBookData(&out.Data)
		default:
			ok = !foldsToKnownKey(key, orderBookTopKeys...) && d.plainScalar()
		}

		if !ok || seen&bit != 0 {
			return false
		}

		seen |= bit

		if d.consume(',') {
			continue
		}

		if d.consume('}') && d.atEnd() {
			break
		}

		return false
	}

	*resp = out

	return true
}

func (d *orderBookDecoder) orderBookData(data *V5WebsocketPublicOrderBookData) bool {
	if !d.consume('{') || d.consume('}') {
		return false
	}

	seen := 0

	for {
		key, ok := d.plainString()
		if !ok || !d.consume(':') {
			return false
		}

		bit := 0

		switch string(key) {
		case "s":
			bit = orderBookDataKeySymbol

			var symbol string
			symbol, ok = d.stringValue()
			data.Symbol = SymbolV5(symbol)
		case "b":
			bit = orderBookDataKeyBids
			data.Bids, ok = d.orderBookLevels()
		case "a":
			bit = orderBookDataKeyAsks

			var asks V5WebsocketPublicOrderBookBids
			asks, ok = d.orderBookLevels()
			data.Asks = V5WebsocketPublicOrderBookAsks(asks)
		case "u":
			bit = orderBookDataKeyUpdateID
			data.UpdateID, ok = d.intValue()
		case "seq":
			bit = orderBookDataKeySeq
			data.Seq, ok = d.intValue()
		default:
			ok = !foldsToKnownKey(key, orderBookDataKeys...) && d.plainScalar()
		}

		if !ok || seen&bit != 0 {
			return false
		}

		seen |= bit

		if d.consume(',') {
			continue
		}

		return d.consume('}')
	}
}

// orderBookLevels returns an empty, not nil, slice for [] as the encoding/json path does.
func (d *orderBookDecoder) orderBookLevels() (V5WebsocketPublicOrderBookBids, bool) {
	if !d.consume('[') {
		return nil, false
	}

	levels := make(V5WebsocketPublicOrderBookBids, 0, d.levelsHint())
	if d.consume(']') {
		return levels, true
	}

	for {
		if !d.consume('[') {
			return nil, false
		}

		priceStart, priceEnd, ok := d.plainStringSpan()
		if !ok || !d.consume(',') {
			return nil, false
		}

		sizeStart, sizeEnd, ok := d.plainStringSpan()
		if !ok || !d.consume(']') {
			return nil, false
		}

		levels = append(levels, struct {
			Price string `json:"price"`
			Size  string `json:"size"`
		}{Price: d.str(priceStart, priceEnd), Size: d.str(sizeStart, sizeEnd)})

		if d.consume(',') {
			continue
		}

		if d.consume(']') {
			return levels, true
		}

		return nil, false
	}
}

const maxLevelsHint = 1000

// levelsHint counts the level openings up to the end of the array; a wrong count only costs a reallocation.
func (d *orderBookDecoder) levelsHint() int {
	n := 0

	for i := d.pos; i < len(d.data) && n < maxLevelsHint; i++ {
		switch d.data[i] {
		case '[':
			n++
		case ']':
			if i == d.pos || d.data[i-1] == ']' {
				return n
			}
		}
	}

	return n
}

// foldsToKnownKey reports a key that encoding/json may match to a known field although it differs from it:
// equal without regard to ASCII case, or after dropping '_' and '-'.
func foldsToKnownKey(key []byte, known ...string) bool {
	for _, k := range known {
		if foldedEqual(key, k) {
			return true
		}
	}

	return false
}

func foldedEqual(key []byte, known string) bool {
	j := 0

	for _, c := range key {
		if c == '_' || c == '-' {
			continue
		}

		if j >= len(known) || lowerASCII(c) != lowerASCII(known[j]) {
			return false
		}

		j++
	}

	return j == len(known)
}

func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}

	return c
}
