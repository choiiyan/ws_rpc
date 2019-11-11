package ws_rpc

import (
	"errors"
	"golang.org/x/crypto/bcrypt"
	"sync"
	"time"
)

type WsServerConf struct {
	port        int64
	path        string
	ticker      int64
	secret      string
	method      []MiddlewareFunc
	waiter      map[string]*Waiter
	closeFunc   CallbackFunc
	connectFunc CallbackFunc
}

type CallbackFunc func(client *Client)

func NewWsRpcServer(port int64, secret string) WsServerConf {
	return WsServerConf{
		port:   port,
		path:   "/",
		ticker: Ticker,
		secret: secret,
		method: make([]MiddlewareFunc, 0),
		waiter: make(map[string]*Waiter),
	}
}

func (s *WsServerConf) MiddlewareFunc(method ...MiddlewareFunc) {
	s.method = append(s.method, method...)
}

func (s *WsServerConf) OnConnectFunc(method CallbackFunc) {
	s.connectFunc = method
}

func (s *WsServerConf) OnCloseFunc(method CallbackFunc) {
	s.closeFunc = method
}

func (s *WsServerConf) RegisterWaiter(waiter string, method interface{}) {
	w := NewWaiter(method)
	s.waiter[stringToLower(waiter)] = w
}

func (s *WsServerConf) Start() error {
	conf := SeverConf{
		Port:   s.port,
		Path:   s.path,
		Ticker: s.ticker,
	}
	hashLoop := NewHashLoop(1000)
	conf.MiddlewareFunc(func(c Context) error {
		token := c.GetFrom()["token"]
		err := bcrypt.CompareHashAndPassword([]byte(token), []byte(s.secret))
		if err != nil {
			return err
		}
		if !hashLoop.Loop(token) {
			return errors.New("key has been used")
		}
		return nil
	})
	conf.MiddlewareFunc(s.method...)
	//启动服务
	err := Start(conf, &wsMethod{
		waiter:      s.waiter,
		closeFunc:   s.closeFunc,
		connectFunc: s.connectFunc,
	})
	return err
}

type wsMethod struct {
	waiter      map[string]*Waiter
	closeFunc   CallbackFunc
	connectFunc CallbackFunc
}

type clientFunc struct {
	look sync.Mutex
	c    chan *resultData
}

var Back = clientFunc{
	look: sync.Mutex{},
	c:    make(chan *resultData),
}

func CallClientFunc(client *Client, waiter, method string, in map[string]interface{}) (map[string]interface{}, error) {
	Back.look.Lock()
	defer Back.look.Unlock()
	rand := getRandString(4)
	res, err := createCallData(waiter, method, rand, in)
	if err != nil {
		return nil, err
	}
	//logger.Debug("CallFunc Msg: Server >>>>>>>>>>>> Client")
	SendMsgToClient(client, res)
	t := time.NewTicker(time.Duration(5) * time.Second)
	defer func() {
		t.Stop()
	}()
	for {
		select {
		case callback := <-Back.c:
			if callback.Random == rand &&
				callback.Waiter == waiter &&
				callback.Method == stringToLower(method) {
				return callback.Out, nil
			}
		case <-t.C:
			return nil, errors.New("call func timeout")
		}
	}
}

func (ws *wsMethod) OnConnect(client *Client) {
	if ws.connectFunc != nil {
		ws.connectFunc(client)
	}
}

//收到信息时处理
func (ws *wsMethod) OnMessage(client *Client, msg []byte) {
	if res, ok := isCallWsFunc(msg); ok {
		//远程调用
		//logger.Debug("CallBack Msg: Server <<<<<<<<<<<< Client")
		out, err := ws.waiter[res.Waiter].RunMethod(res.Method, client, res.In)
		m, err := createResultData(res, out, err)
		if err != nil {
			return
		}
		//logger.Debug("CallBack Msg: Server >>>>>>>>>>>> Client")
		client.SendMsg(m)
	} else if res, ok := isResWsFunc(msg); ok {
		Back.c <- res
		//logger.Debug("CallFunc Msg: Server <<<<<<<<<<<< Client")
	}
}

//断开连接
func (ws *wsMethod) OnClose(client *Client) {
	if ws.closeFunc != nil {
		ws.closeFunc(client)
	}
}
