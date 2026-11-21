package bybit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// runOutcome is what Run makes of a frame: an error, or the topic and, for an order book, the decoded response.
type runOutcome struct {
	err   bool
	topic V5WebsocketPublicTopic
	book  V5WebsocketPublicOrderBookResponse
}

func decodeTyped(topic V5WebsocketPublicTopic, message []byte, category CategoryV5, fast bool, out *runOutcome) {
	var err error

	switch topic {
	case V5WebsocketPublicTopicOrderBook:
		if fast {
			err = DecodeV5WebsocketPublicOrderBook(message, &out.book)
		} else {
			err = json.Unmarshal(message, &out.book)
		}
	case V5WebsocketPublicTopicKline:
		var r V5WebsocketPublicKlineResponse
		err = json.Unmarshal(message, &r)
	case V5WebsocketPublicTopicTicker:
		var r V5WebsocketPublicTickerResponse
		r.Data.category = category
		err = json.Unmarshal(message, &r)
	case V5WebsocketPublicTopicTrade:
		var r V5WebsocketPublicTradeResponse
		err = json.Unmarshal(message, &r)
	case V5WebsocketPublicTopicLiquidation:
		var r V5WebsocketPublicLiquidationResponse
		err = json.Unmarshal(message, &r)
	}

	out.err = err != nil
}

func legacyRun(message []byte, category CategoryV5) runOutcome {
	var out runOutcome

	topic, err := judgeTopicJSON(message)
	if err != nil {
		out.err = true
		return out
	}

	out.topic = topic
	decodeTyped(topic, message, category, false, &out)

	return out
}

func currentRun(message []byte, category CategoryV5) runOutcome {
	var out runOutcome

	topic, err := JudgeV5WebsocketPublicTopic(message)
	if err != nil {
		out.err = true
		return out
	}

	out.topic = topic
	decodeTyped(topic, message, category, true, &out)

	return out
}

func requireRunMatchesLegacy(t *testing.T, message []byte) {
	t.Helper()

	shown := message
	if len(shown) > 300 {
		shown = shown[:300]
	}

	for _, category := range []CategoryV5{CategoryV5Spot, CategoryV5Linear} {
		want, got := legacyRun(message, category), currentRun(message, category)
		require.Equal(t, want.err, got.err, "error outcome, frame %q (len %d)", shown, len(message))

		if !want.err {
			require.Equal(t, want, got, "frame %q", shown)
		}
	}
}

func runBoundarySeeds() []string {
	deep := strings.Repeat("[", 10001) + strings.Repeat("]", 10001)
	deepO := strings.Repeat(`{"a":`, 10001) + "1" + strings.Repeat("}", 10001)
	s := []string{}
	for _, tp := range []string{"orderbook.50.X", "kline.1.X", "tickers.X", "publicTrade.X", "liquidation.X"} {
		p := `{"topic":"` + tp + `",`
		s = append(s,
			p+`"a":`+deep+`}`,
			p+`"a":`+deepO+`}`,
			p+`"a":`+strings.Repeat("[", 9999)+strings.Repeat("]", 9999)+`}`,
			p+`"a":1`+strings.Repeat("0", 299)+`}`,
			p+`"a":1`+strings.Repeat("0", 300)+`}`,
			p+`"a":1`+strings.Repeat("0", 308)+`}`,
			p+`"a":1`+strings.Repeat("0", 309)+`}`,
			p+`"a":[1`+strings.Repeat("0", 309)+`]}`,
			p+`"a":0.`+strings.Repeat("0", 400)+`1}`,
			p+`"a":-1`+strings.Repeat("0", 400)+`}`,
			p+`"a":1e308}`, p+`"a":1e309}`, p+`"a":[1E+400]}`, p+`"a":{"b":2e400}}`,
			p+`"a":"\ud800"}`, p+"\"a\":\"\xff\"}", p+"\"a\":\"\x01\"}", p+"\"a\":\"\t\"}",
			p+`"a":"\x"}`, p+`"a":"\u12"}`, p+`"a":[}`, p+`"a":{]}`, p+`"a":[1,]}`, p+`"a":{"b"}}`,
			p+`"a":tru}`, p+`"a":nul}`, p+`"a":-}`, p+`"a":01}`, p+`"a":1.}`, p+`"a":.5}`, p+`"a":+1}`,
			p+`"a":1 }`, p+`"a": 1}`, p+`"a":[1 , 2]}`, p+`"a":1,"a":2}`, p+`"a":{"topic":"x","topic":1}}`,
			p+`"a":"x`+"\x7f"+`"}`, p+`"a":NaN}`, p+`"a":Infinity}`, p+`"a":0x10}`, p+`"a":1_0}`,
			p+`"a":[1]]}`, p+`"a":"\\"}`, p+`"a":"\\\""}`, p+`"a":'x'}`, p+`"a":/**/1}`,
			p+`"data":{"s":"X","b":[["1","2"]],"a":[["3","4"]],"u":1,"seq":2},"ts":5,"type":"delta"}`,
			p+`"data":{"s":"X","b":[["1","2"]],"u":-0,"seq":2},"ts":5}`,
			p+`"data":{"s":"X","b":[["[","]]"]],"u":0,"seq":2},"ts":0}`,
			p+`"data":{"symbol":"X","lastPrice":"1"},"ts":5,"type":"delta","cs":1}`,
			p+`"data":[{"start":1,"T":2,"s":"X"}],"ts":5}`,
			`{"topic":"`+tp+`","topic":"kline.1.Y"}`,
			`{"topic":"kline.1.Y","topic":"`+tp+`"}`,
			`{"topic":"kline.1.Y","Topic":"`+tp+`"}`,
			`{"TOPIC":"`+tp+`"}`,
			`{"topic":"`+tp+`","topic":null}`,
			`{"topic":null,"topic":"`+tp+`"}`,
			`{"topic":"`+tp+`"`+"\n}",
		)
	}
	return s
}

func TestRun_MatchesLegacy(t *testing.T) {
	t.Parallel()

	inputs := append(topicCases(), runBoundarySeeds()...)
	accepted, refused := orderBookCases()
	inputs = append(append(inputs, accepted...), refused...)

	for _, input := range inputs {
		requireRunMatchesLegacy(t, []byte(input))
	}

	for _, frame := range recordedFrames(t) {
		requireRunMatchesLegacy(t, frame)
	}
}

func FuzzRun_MatchesLegacy(f *testing.F) {
	accepted, refused := orderBookCases()
	for _, input := range append(append(topicCases(), accepted...), refused...) {
		f.Add([]byte(input))
	}

	for _, input := range runBoundarySeeds() {
		if len(input) < 2000 {
			f.Add([]byte(input))
		}
	}

	f.Fuzz(func(t *testing.T, message []byte) {
		requireRunMatchesLegacy(t, message)
	})
}
