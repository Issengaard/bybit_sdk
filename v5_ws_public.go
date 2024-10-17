package bybit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// V5WebsocketPublicServiceI :
type V5WebsocketPublicServiceI interface {
	Start(context.Context, ErrHandler) error
	Run() error
	Ping() error
	Close() error

	SubscribeOrderBook(
		V5WebsocketPublicOrderBookParamKey,
		func(V5WebsocketPublicOrderBookResponse) error,
	) (func() error, error)

	SubscribeKline(
		V5WebsocketPublicKlineParamKey,
		func(V5WebsocketPublicKlineResponse) error,
	) (func() error, error)

	SubscribeTicker(
		V5WebsocketPublicTickerParamKey,
		func(V5WebsocketPublicTickerResponse) error,
	) (func() error, error)

	SubscribeTrade(
		V5WebsocketPublicTradeParamKey,
		func(V5WebsocketPublicTradeResponse) error,
	) (func() error, error)

	SubscribeLiquidation(
		V5WebsocketPublicLiquidationParamKey,
		func(V5WebsocketPublicLiquidationResponse) error,
	) (func() error, error)
}

// V5WebsocketPublicService :
type V5WebsocketPublicService struct {
	client     *WebSocketClient
	connection *websocket.Conn
	category   CategoryV5

	connectionWritMutex sync.Mutex

	paramOrderBookMap   *PublicWsHandlersMap[V5WebsocketPublicOrderBookParamKey, V5WebsocketPublicOrderBookResponse]
	paramKlineMap       *PublicWsHandlersMap[V5WebsocketPublicKlineParamKey, V5WebsocketPublicKlineResponse]
	paramTickerMap      *PublicWsHandlersMap[V5WebsocketPublicTickerParamKey, V5WebsocketPublicTickerResponse]
	paramTradeMap       *PublicWsHandlersMap[V5WebsocketPublicTradeParamKey, V5WebsocketPublicTradeResponse]
	paramLiquidationMap *PublicWsHandlersMap[V5WebsocketPublicLiquidationParamKey, V5WebsocketPublicLiquidationResponse]
}

const (
	// V5WebsocketPublicPath :
	V5WebsocketPublicPath = "/v5/public"
)

// V5WebsocketPublicPathFor :
func V5WebsocketPublicPathFor(category CategoryV5) string {
	return V5WebsocketPublicPath + "/" + string(category)
}

// V5WebsocketPublicTopic :
type V5WebsocketPublicTopic string

const (
	// V5WebsocketPublicTopicOrderBook :
	V5WebsocketPublicTopicOrderBook = V5WebsocketPublicTopic("orderbook")

	// V5WebsocketPublicTopicKline :
	V5WebsocketPublicTopicKline = V5WebsocketPublicTopic("kline")

	// V5WebsocketPublicTopicTicker :
	V5WebsocketPublicTopicTicker = V5WebsocketPublicTopic("tickers")

	// V5WebsocketPublicTopicTrade :
	V5WebsocketPublicTopicTrade = V5WebsocketPublicTopic("publicTrade")

	// V5WebsocketPublicTopicLiquidation :
	V5WebsocketPublicTopicLiquidation = V5WebsocketPublicTopic("liquidation")
)

func (t V5WebsocketPublicTopic) String() string {
	return string(t)
}

// judgeTopic :
func (s *V5WebsocketPublicService) judgeTopic(respBody []byte) (V5WebsocketPublicTopic, error) {
	parsedData := map[string]interface{}{}
	if err := json.Unmarshal(respBody, &parsedData); err != nil {
		return "", err
	}
	if topic, ok := parsedData["topic"].(string); ok {
		switch {
		case strings.Contains(topic, V5WebsocketPublicTopicOrderBook.String()):
			return V5WebsocketPublicTopicOrderBook, nil
		case strings.Contains(topic, V5WebsocketPublicTopicKline.String()):
			return V5WebsocketPublicTopicKline, nil
		case strings.Contains(topic, V5WebsocketPublicTopicTicker.String()):
			return V5WebsocketPublicTopicTicker, nil
		case strings.Contains(topic, V5WebsocketPublicTopicTrade.String()):
			return V5WebsocketPublicTopicTrade, nil
		case strings.Contains(topic, V5WebsocketPublicTopicLiquidation.String()):
			return V5WebsocketPublicTopicLiquidation, nil
		}
	}
	return "", nil
}

// UnmarshalJSON :
func (r *V5WebsocketPublicTickerData) UnmarshalJSON(data []byte) error {
	switch r.category {
	case CategoryV5Linear, CategoryV5Inverse:
		return json.Unmarshal(data, &r.LinearInverse)
	case CategoryV5Option:
		return json.Unmarshal(data, &r.Option)
	case CategoryV5Spot:
		return json.Unmarshal(data, &r.Spot)
	}
	return errors.New("unsupported format")
}

// parseResponse :
func (s *V5WebsocketPublicService) parseResponse(respBody []byte, response interface{}) error {
	if err := json.Unmarshal(respBody, &response); err != nil {
		return err
	}
	return nil
}

// Start :
func (s *V5WebsocketPublicService) Start(ctx context.Context, errHandler ErrHandler) error {
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer s.connection.Close()

		_ = s.connection.SetReadDeadline(time.Now().Add(60 * time.Second))
		s.connection.SetPongHandler(func(string) error {
			_ = s.connection.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})

		for {
			if err := s.Run(); err != nil {
				if errHandler == nil {
					return
				}
				errHandler(IsErrWebsocketClosed(err), err)
				return
			}
		}
	}()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	for {
		select {
		case <-done:
			return nil
		case <-ticker.C:
			if err := s.Ping(); err != nil {
				return err
			}
		case <-ctx.Done():
			s.client.debugf("caught websocket public service interrupt signal")

			if err := s.Close(); err != nil {
				return err
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return nil
		}
	}
}

// Run :
func (s *V5WebsocketPublicService) Run() error {
	_, message, err := s.connection.ReadMessage()
	if err != nil {
		return err
	}

	topic, err := s.judgeTopic(message)
	if err != nil {
		return err
	}
	switch topic {
	case V5WebsocketPublicTopicOrderBook:
		return s.handleWebsocketPublicTopicOrderBook(message)

	case V5WebsocketPublicTopicKline:
		return s.handleWebsocketPublicTopicKline(message)

	case V5WebsocketPublicTopicTicker:
		return s.handleWebsocketPublicTopicTicker(message)

	case V5WebsocketPublicTopicTrade:
		return s.handleWebsocketPublicTopicTrade(message)

	case V5WebsocketPublicTopicLiquidation:
		return s.handleWebsocketPublicTopicLiquidation(message)

	}
	return nil
}

// Ping :
func (s *V5WebsocketPublicService) Ping() error {
	// NOTE: It appears that two messages need to be sent.
	// REF: https://github.com/hirokisan/bybit/pull/127#issuecomment-1537479346
	if err := s.writeMessage(websocket.PingMessage, nil); err != nil {
		return err
	}
	if err := s.writeMessage(websocket.TextMessage, []byte(`{"op":"ping"}`)); err != nil {
		return err
	}
	return nil
}

// Close :
func (s *V5WebsocketPublicService) Close() error {
	if err := s.writeMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil && !errors.Is(err, websocket.ErrCloseSent) {
		return err
	}
	return nil
}

func (s *V5WebsocketPublicService) handleWebsocketPublicTopicOrderBook(message []byte) error {
	var resp V5WebsocketPublicOrderBookResponse
	if err := s.parseResponse(message, &resp); err != nil {
		return err
	}

	// When a handler function is deleted and a stop request is sent to the server,
	// data messages may still be received. To prevent connection restarts in this scenario,
	// errors are not returned in this block of code.
	f, isExist := s.retrieveOrderBookFunc(resp.Key())
	if !isExist {
		return nil
	}

	return f(resp)
}

func (s *V5WebsocketPublicService) handleWebsocketPublicTopicKline(message []byte) error {
	var resp V5WebsocketPublicKlineResponse
	if err := s.parseResponse(message, &resp); err != nil {
		return err
	}

	// When a handler function is deleted and a stop request is sent to the server,
	// data messages may still be received. To prevent connection restarts in this scenario,
	// errors are not returned in this block of code.
	f, isExist := s.retrieveKlineFunc(resp.Key())
	if !isExist {
		return nil
	}

	return f(resp)
}

func (s *V5WebsocketPublicService) handleWebsocketPublicTopicTicker(message []byte) error {
	var resp V5WebsocketPublicTickerResponse
	resp.Data.category = s.category
	if err := s.parseResponse(message, &resp); err != nil {
		return err
	}

	// When a handler function is deleted and a stop request is sent to the server,
	// data messages may still be received. To prevent connection restarts in this scenario,
	// errors are not returned in this block of code.
	f, isExist := s.retrieveTickerFunc(resp.Key())
	if !isExist {
		return nil
	}

	return f(resp)
}

func (s *V5WebsocketPublicService) handleWebsocketPublicTopicTrade(message []byte) error {
	var resp V5WebsocketPublicTradeResponse
	if err := s.parseResponse(message, &resp); err != nil {
		return err
	}

	// When a handler function is deleted and a stop request is sent to the server,
	// data messages may still be received. To prevent connection restarts in this scenario,
	// errors are not returned in this block of code.
	f, isExist := s.retrieveTradeFunc(resp.Key())
	if !isExist {
		return nil
	}

	return f(resp)
}

func (s *V5WebsocketPublicService) handleWebsocketPublicTopicLiquidation(message []byte) error {
	var resp V5WebsocketPublicLiquidationResponse
	if err := s.parseResponse(message, &resp); err != nil {
		return err
	}

	// When a handler function is deleted and a stop request is sent to the server,
	// data messages may still be received. To prevent connection restarts in this scenario,
	// errors are not returned in this block of code.
	f, isExist := s.retrieveLiquidationFunc(resp.Key())
	if !isExist {
		return nil
	}

	return f(resp)
}

func (s *V5WebsocketPublicService) writeMessage(messageType int, body []byte) error {
	s.connectionWritMutex.Lock()
	defer s.connectionWritMutex.Unlock()

	if err := s.connection.WriteMessage(messageType, body); err != nil {
		return err
	}
	return nil
}
