package bybit

import (
	"github.com/gorilla/websocket"
)

// V5WebsocketServiceI :
type V5WebsocketServiceI interface {
	Public(CategoryV5) (V5WebsocketPublicService, error)
	Private() (V5WebsocketPrivateService, error)
	Trade() (V5WebsocketTradeService, error)
}

// V5WebsocketService :
type V5WebsocketService struct {
	client *WebSocketClient
}

// Public :
func (s *V5WebsocketService) Public(category CategoryV5) (V5WebsocketPublicServiceI, error) {
	url := s.client.baseURL + V5WebsocketPublicPathFor(category)
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	return &V5WebsocketPublicService{
		client:              s.client,
		connection:          c,
		category:            category,
		paramOrderBookMap:   NewPublicWsHandlersMap[V5WebsocketPublicOrderBookParamKey, V5WebsocketPublicOrderBookResponse](),
		paramKlineMap:       NewPublicWsHandlersMap[V5WebsocketPublicKlineParamKey, V5WebsocketPublicKlineResponse](),
		paramTickerMap:      NewPublicWsHandlersMap[V5WebsocketPublicTickerParamKey, V5WebsocketPublicTickerResponse](),
		paramTradeMap:       NewPublicWsHandlersMap[V5WebsocketPublicTradeParamKey, V5WebsocketPublicTradeResponse](),
		paramLiquidationMap: NewPublicWsHandlersMap[V5WebsocketPublicLiquidationParamKey, V5WebsocketPublicLiquidationResponse](),
	}, nil
}

// Private :
func (s *V5WebsocketService) Private() (V5WebsocketPrivateServiceI, error) {
	url := s.client.baseURL + V5WebsocketPrivatePath
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	return &V5WebsocketPrivateService{
		client:            s.client,
		connection:        c,
		paramOrderMap:     NewPublicWsHandlersMap[V5WebsocketPrivateParamKey, V5WebsocketPrivateOrderResponse](),
		paramPositionMap:  NewPublicWsHandlersMap[V5WebsocketPrivateParamKey, V5WebsocketPrivatePositionResponse](),
		paramExecutionMap: NewPublicWsHandlersMap[V5WebsocketPrivateParamKey, V5WebsocketPrivateExecutionResponse](),
		paramWalletMap:    NewPublicWsHandlersMap[V5WebsocketPrivateParamKey, V5WebsocketPrivateWalletResponse](),
	}, nil
}

// Trade :
func (s *V5WebsocketService) Trade() (V5WebsocketTradeServiceI, error) {
	url := s.client.baseURL + V5WebsocketTradePath
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	return &V5WebsocketTradeService{
		client:     s.client,
		connection: c,
	}, nil
}

// V5 :
func (c *WebSocketClient) V5() *V5WebsocketService {
	return &V5WebsocketService{c}
}
