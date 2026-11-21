package bybit

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Issengaard/bybit_sdk/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sampleSpotDelta   = `{"topic":"orderbook.50.BTCUSDT","ts":1795220992788,"type":"delta","data":{"s":"BTCUSDT","b":[["59063.2","0.682627"],["59063.1","0"]],"a":[["59063.3","1.5"]],"u":1234,"seq":98765},"cts":1795220992780}`
	sampleLinearDelta = `{"topic":"orderbook.50.ETHUSDT","type":"delta","ts":1795220992788,"data":{"s":"ETHUSDT","b":[],"a":[["1750.83","14.59"]],"u":55,"seq":66},"cts":1795220992780}`
	sampleTop1        = `{"topic":"orderbook.1.BTCUSDT","ts":1795220992616,"type":"snapshot","data":{"s":"BTCUSDT","b":[["59062.9","0.1"]],"a":[["59063","0.2"]],"u":1,"seq":2},"cts":1795220992610}`
	sampleTicker      = `{"topic":"tickers.BTCUSDT","ts":1795220992616,"type":"snapshot","cs":118242802445,"data":{"symbol":"BTCUSDT","lastPrice":"59062.9"}}`
	sampleKlineSpot   = `{"type":"snapshot","topic":"kline.1.BTCUSDT","data":[{"start":1795220940000,"confirm":false}],"ts":1795220992616}`
	sampleTrade       = `{"topic":"publicTrade.BTCUSDT","type":"snapshot","ts":1795220992616,"data":[{"T":1795220992615,"s":"BTCUSDT","S":"Buy","v":"0.001","p":"59062.9","BT":false}]}`
	sampleLiquidation = `{"topic":"liquidation.BTCUSDT","type":"snapshot","ts":1795220992616,"data":{"price":"1","side":"Buy","size":"2","symbol":"BTCUSDT","updatedTime":3}}`
	sampleSubscribed  = `{"success":true,"ret_msg":"subscribe","conn_id":"x-1","op":"subscribe"}`
	sampleSubFailed   = `{"success":false,"ret_msg":"Invalid symbol :[orderbook.50.NOPE]","conn_id":"x-1","op":"subscribe"}`
	samplePong        = `{"success":true,"ret_msg":"pong","conn_id":"x-1","req_id":"","op":"ping"}`
)

func recordedFrames(tb testing.TB) [][]byte {
	tb.Helper()

	var frames [][]byte

	for _, name := range []string{"testdata/ws_public_spot.jsonl.gz", "testdata/ws_public_linear.jsonl.gz"} {
		f, err := os.Open(name)
		require.NoError(tb, err)

		gz, err := gzip.NewReader(f)
		require.NoError(tb, err)

		sc := bufio.NewScanner(gz)
		sc.Buffer(make([]byte, 1<<20), 1<<24)

		for sc.Scan() {
			frames = append(frames, append([]byte(nil), sc.Bytes()...))
		}

		require.NoError(tb, sc.Err())
		require.NoError(tb, f.Close())
	}

	require.NotEmpty(tb, frames)

	return frames
}

func legacyOrderBookKey(r *V5WebsocketPublicOrderBookResponse) V5WebsocketPublicOrderBookParamKey {
	arr := strings.Split(r.Topic, ".")
	if arr[0] != V5WebsocketPublicTopicOrderBook.String() || len(arr) != 3 {
		return V5WebsocketPublicOrderBookParamKey{}
	}

	depth, err := strconv.Atoi(arr[1])
	if err != nil {
		return V5WebsocketPublicOrderBookParamKey{}
	}

	return V5WebsocketPublicOrderBookParamKey{Depth: depth, Symbol: SymbolV5(arr[2])}
}

func assertTopicMatchesLegacy(t *testing.T, message []byte) {
	t.Helper()

	got, gotErr := JudgeV5WebsocketPublicTopic(message)
	want, wantErr := judgeTopicJSON(message)

	if wantErr == nil {
		require.NoError(t, gotErr, "frame %q", message)
		require.Equal(t, want, got, "frame %q", message)

		return
	}

	if gotErr != nil {
		return
	}

	require.NotEmpty(t, got, "frame %q", message)
	require.False(t, json.Valid(message), "frame %q", message)

	if got == V5WebsocketPublicTopicOrderBook {
		var resp V5WebsocketPublicOrderBookResponse
		require.Error(t, DecodeV5WebsocketPublicOrderBook(message, &resp), "frame %q", message)
	}
}

func assertOrderBookMatchesLegacy(t *testing.T, message []byte) {
	t.Helper()

	var fast V5WebsocketPublicOrderBookResponse

	fastOK := decodeOrderBookFast(message, &fast)

	var want V5WebsocketPublicOrderBookResponse

	wantErr := json.Unmarshal(message, &want)

	if fastOK {
		require.NoError(t, wantErr, "frame %q", message)
		require.Equal(t, want, fast, "frame %q", message)
	} else {
		require.Equal(t, V5WebsocketPublicOrderBookResponse{}, fast, "frame %q", message)
	}

	var got V5WebsocketPublicOrderBookResponse

	gotErr := DecodeV5WebsocketPublicOrderBook(message, &got)

	require.Equal(t, wantErr != nil, gotErr != nil, "frame %q", message)

	if wantErr == nil {
		require.Equal(t, want, got, "frame %q", message)
	}
}

func topicCases() []string {
	cases := []string{
		sampleSpotDelta, sampleLinearDelta, sampleTop1, sampleTicker, sampleKlineSpot, sampleTrade,
		sampleLiquidation, sampleSubscribed, sampleSubFailed, samplePong,
		`{"a":1,"topic":"tickers.X"}`,
		`{}`, ``, `[]`, `"x"`, `1`, `null`,
		`{"topic":1}`,
		`{"topic":"unknown.X"}`,
		`{"topic":"tickers.X","topic":"orderbook.1.X"}`,
		`{"topic":"orderbook.1.X","topic":"nothing"}`,
		`{"to":"orderbook","topico":"kline.1.X"}`,
		`{ "topic" : "orderbook.1.X" }`,
		`{"topic":"orderbook.1.X"} `,
		`{"data":{"topic":"orderbook.1.X"},"x":1}`,
		`{"topic":"orderbook.1.X"}`,
		`{"topic":"orderbook.1.X"}`,
		`{"a":tru,"topic":"orderbook.1.X"}`,
		`{"a":[1,{"b":"]"}],"topic":"kline.1.X"}`,
		`{"a":"\"","topic":"kline.1.X"}`,
		`{"topic":"orderbook.1.X",}`,
		`{"topic":"orderbook.1.X"}}`,
		"{\"topic\":\"orderbook.1.\xff\"}",
		`{"topic":"kline","":1E700}`,
		`{"topic":"kline","":[{"a":1e700}]}`,
		`{"topic":"kline","":1` + strings.Repeat("0", 400) + `}`,
		`{"topic":"kline","":1.5e3}`,
	}

	for _, sample := range []string{sampleSpotDelta, sampleKlineSpot, sampleSubscribed} {
		for i := 0; i < len(sample); i++ {
			cases = append(cases, sample[:i])
		}
	}

	return cases
}

func TestScanTopic(t *testing.T) {
	t.Parallel()

	for i, tc := range topicCases() {
		tc := tc

		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()

			assertTopicMatchesLegacy(t, []byte(tc))
		})
	}
}

func TestScanTopic_Corpus(t *testing.T) {
	t.Parallel()

	counts := map[V5WebsocketPublicTopic]int{}

	for _, frame := range recordedFrames(t) {
		topic, ok := scanTopic(frame)
		if bytes.Contains(frame, []byte(`"topic"`)) {
			require.True(t, ok, "frame %q", frame)
		}

		assertTopicMatchesLegacy(t, frame)

		counts[topic]++
	}

	for _, topic := range []V5WebsocketPublicTopic{
		V5WebsocketPublicTopicOrderBook, V5WebsocketPublicTopicTicker, V5WebsocketPublicTopicKline, V5WebsocketPublicTopicTrade, "",
	} {
		assert.Positive(t, counts[topic], "topic %q", topic)
	}
}

func orderBookCases() (accepted, refused []string) {
	accepted = []string{
		sampleSpotDelta,
		sampleLinearDelta,
		sampleTop1,
		`{"topic":"orderbook.50.X","type":"snapshot","ts":1,"data":{"s":"X","b":[],"a":[],"u":1,"seq":0}}`,
		`{"topic":"orderbook.50.X","type":"delta","ts":1,"data":{"s":"X","a":[["1","2"]],"u":1,"seq":0}}`,
		`{"topic":"orderbook.50.X","type":"delta","ts":1}`,
		`{"topic":"orderbook.50.X","type":"delta","ts":1,"data":{"s":"X","b":[["1","2"]],"a":[["3","4"]],"u":1,"seq":2}}`,
		`{"data":{"seq":2,"u":1,"a":[["3","4"]],"b":[["1","2"]],"s":"X"},"ts":0,"type":"delta","topic":"orderbook.50.X"}`,
		`{"topic":"orderbook.50.X","extra":true,"other":null,"num":7,"str":"v","data":{"s":"X","z":false,"u":1}}`,
	}

	refused = []string{
		`{"topic": "orderbook.50.X"}`,
		`{"topic":"orderbook.50.X","type":"de\"lta"}`,
		`{"topic":"orderbook.50.X","ts":"1"}`,
		`{"topic":"orderbook.50.X","ts":1.0}`,
		`{"topic":"orderbook.50.X","ts":1e3}`,
		`{"topic":"orderbook.50.X","ts":-1}`,
		`{"topic":"orderbook.50.X","ts":01}`,
		`{"topic":"orderbook.50.X","ts":1234567890123456789}`,
		`{"topic":"orderbook.50.X","data":{"s":"X","b":null}}`,
		`{"topic":"orderbook.50.X","data":null}`,
		`{"topic":"orderbook.50.X","data":{}}`,
		`{"topic":"orderbook.50.X","data":{"s":"X","b":[["1"]]}}`,
		`{"topic":"orderbook.50.X","data":{"s":"X","b":[["1","2","3"]]}}`,
		`{"topic":"orderbook.50.X","data":{"s":"X","b":[[1,"2"]]}}`,
		`{"topic":"orderbook.50.X","ts":1,"ts":2}`,
		`{"topic":"orderbook.50.X","data":{"s":"X","b":[["1","2"]],"b":[]}}`,
		`{"topic":"orderbook.50.X","TS":2}`,
		`{"topic":"orderbook.50.X","t_s":2}`,
		`{"topic":"orderbook.50.X","data":{"s":"X","B":[]}}`,
		`{"topic":"orderbook.50.X","obj":{"a":1}}`,
		`{"topic":"orderbook.50.X","arr":[1]}`,
		`{"topic":"orderbook.50.X"}x`,
		"{\"topic\":\"orderbook.50.X\",\"data\":{\"s\":\"\xc3\xa9\"}}",
		`{"topic":"orderbook.50.X","data":{"s":"X","b":[["1","2"],]}}`,
		`{"topic":"orderbook.50.X","data":{"s":"X","u":1.5}}`,
		`{"topic":"orderbook.50.X","data":{"s":"X"},}`,
		`[]`,
		``,
	}

	return accepted, refused
}

func TestDecodeOrderBookFast(t *testing.T) {
	t.Parallel()

	accepted, refused := orderBookCases()

	for i, tc := range accepted {
		tc := tc

		t.Run("accepted_"+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()

			var resp V5WebsocketPublicOrderBookResponse
			require.True(t, decodeOrderBookFast([]byte(tc), &resp))
			assertOrderBookMatchesLegacy(t, []byte(tc))
		})
	}

	for i, tc := range refused {
		tc := tc

		t.Run("refused_"+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()

			resp := V5WebsocketPublicOrderBookResponse{Topic: "untouched"}
			require.False(t, decodeOrderBookFast([]byte(tc), &resp))
			require.Equal(t, V5WebsocketPublicOrderBookResponse{Topic: "untouched"}, resp)
			assertOrderBookMatchesLegacy(t, []byte(tc))
		})
	}
}

func TestDecodeOrderBookFast_Corpus(t *testing.T) {
	t.Parallel()

	n := 0

	for _, frame := range recordedFrames(t) {
		if !bytes.HasPrefix(frame, []byte(`{"topic":"orderbook.`)) {
			continue
		}

		var resp V5WebsocketPublicOrderBookResponse
		require.True(t, decodeOrderBookFast(frame, &resp), "frame %q", frame)
		assertOrderBookMatchesLegacy(t, frame)

		n++
	}

	require.Positive(t, n)
}

func TestOrderBookKey_MatchesLegacy(t *testing.T) {
	t.Parallel()

	topics := []string{
		"orderbook.50.BTCUSDT", "orderbook.1.BTCUSDT", "orderbook.1.MNTUSDT-27NOV26",
		"", ".", "..", "...", "orderbook", "orderbook.", "orderbook.50", "orderbook.50.", "orderbook..X",
		"orderbook.x.X", "orderbook.-1.X", "orderbook.50.X.Y", "orderbook.50.X.Y.Z", "tickers.50.X", ".50.X",
		"orderbookx.50.X", "orderbook.+5.X",
	}

	for _, topic := range topics {
		topic := topic

		t.Run(topic, func(t *testing.T) {
			t.Parallel()

			r := V5WebsocketPublicOrderBookResponse{Topic: topic}
			assert.Equal(t, legacyOrderBookKey(&r), r.Key())
		})
	}
}

func FuzzScanTopic(f *testing.F) {
	for _, tc := range topicCases() {
		f.Add([]byte(tc))
	}

	f.Fuzz(func(t *testing.T, message []byte) {
		assertTopicMatchesLegacy(t, message)
	})
}

func FuzzDecodeOrderBookFast(f *testing.F) {
	accepted, refused := orderBookCases()
	for _, tc := range append(accepted, refused...) {
		f.Add([]byte(tc))
	}

	f.Fuzz(func(t *testing.T, message []byte) {
		assertOrderBookMatchesLegacy(t, message)
	})
}

func TestWebsocketV5Public_OrderBook_FastAndFallback(t *testing.T) {
	t.Parallel()

	var indented bytes.Buffer
	require.NoError(t, json.Indent(&indented, []byte(sampleSpotDelta), "", "  "))

	tests := []struct {
		name    string
		frame   []byte
		wantErr bool
	}{
		{name: "compact", frame: []byte(sampleSpotDelta)},
		{name: "indented", frame: indented.Bytes()},
		{name: "truncated", frame: []byte(sampleSpotDelta[:len(sampleSpotDelta)-10]), wantErr: true},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			category := CategoryV5Spot

			server, teardown := testhelper.NewWebsocketServer(
				testhelper.WithWebsocketHandlerOption(V5WebsocketPublicPathFor(category), tt.frame),
			)
			defer teardown()

			svc, err := NewTestWebsocketClient().WithBaseURL(server.URL).V5().Public(category)
			require.NoError(t, err)

			var got []V5WebsocketPublicOrderBookResponse

			_, err = svc.SubscribeOrderBook(
				V5WebsocketPublicOrderBookParamKey{Depth: 50, Symbol: SymbolV5BTCUSDT},
				func(response V5WebsocketPublicOrderBookResponse) error {
					got = append(got, response)
					return nil
				},
			)
			require.NoError(t, err)

			err = svc.Run()
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)

				return
			}

			require.NoError(t, err)
			require.Len(t, got, 1)
			assert.Equal(t, "orderbook.50.BTCUSDT", got[0].Topic)
			assert.Equal(t, "delta", got[0].Type)
			assert.Equal(t, int64(1795220992788), got[0].TimeStamp)
			assert.Equal(t, SymbolV5BTCUSDT, got[0].Data.Symbol)
			assert.Equal(t, 1234, got[0].Data.UpdateID)
			assert.Equal(t, 98765, got[0].Data.Seq)
			require.Len(t, got[0].Data.Bids, 2)
			assert.Equal(t, "59063.1", got[0].Data.Bids[1].Price)
			assert.Equal(t, "0", got[0].Data.Bids[1].Size)
			require.Len(t, got[0].Data.Asks, 1)
			assert.Equal(t, "1.5", got[0].Data.Asks[0].Size)
			assert.NoError(t, svc.Close())
		})
	}
}

func BenchmarkJudgeTopic(b *testing.B) {
	frames := recordedFrames(b)

	b.Run("json", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, _ = judgeTopicJSON(frames[i%len(frames)])
		}
	})

	b.Run("scan", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, _ = JudgeV5WebsocketPublicTopic(frames[i%len(frames)])
		}
	})
}

func BenchmarkDecodeOrderBook(b *testing.B) {
	var frames [][]byte

	for _, frame := range recordedFrames(b) {
		if bytes.HasPrefix(frame, []byte(`{"topic":"orderbook.`)) {
			frames = append(frames, frame)
		}
	}

	b.Run("json", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			var resp V5WebsocketPublicOrderBookResponse
			_ = json.Unmarshal(frames[i%len(frames)], &resp)
		}
	})

	b.Run("fast", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			var resp V5WebsocketPublicOrderBookResponse
			_ = DecodeV5WebsocketPublicOrderBook(frames[i%len(frames)], &resp)
		}
	})
}
