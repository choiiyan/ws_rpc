package ws_rpc

import (
	"errors"
	"golang.org/x/crypto/bcrypt"
	"log"
	"sync"
	"time"
)

type DisconnectFunc func(w *WSRpcClient)

type WSRpcClient struct {
	client    WSClient
	conf      ClientConf
	waiter    map[string]*Waiter
	back      chan *resultData
	call      chan *callData
	callClose chan bool
	lock      sync.Mutex
	disMethod []DisconnectFunc
	err       chan error
	isClose   bool
}

func NewWsRpcClient(host string, secret string) *WSRpcClient {
	conf := ClientConf{
		Host:   host,
		Path:   "/",
		Ticker: Ticker,
	}
	rpcClient := new(WSRpcClient)
	hashed, _ := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if secret != "" {
		conf.Path = conf.Path + "?token=" + string(hashed)
	}
	rpcClient.conf = conf
	rpcClient.waiter = make(map[string]*Waiter)
	rpcClient.isClose = false
	return rpcClient
}

func (w *WSRpcClient) RegisterWaiter(waiter string, method interface{}) *WSRpcClient {
	m := NewWaiter(method)
	w.waiter[stringToLower(waiter)] = m
	return w
}

func (w *WSRpcClient) DisconnectFunc(method ...DisconnectFunc) *WSRpcClient {
	w.disMethod = append(w.disMethod, method...)
	return w
}

func (w *WSRpcClient) Start() (*WSRpcClient, error) {
	w.back = make(chan *resultData)
	w.call = make(chan *callData)
	w.callClose = make(chan bool, 1)
	w.err = make(chan error, 1)
	w.lock = sync.Mutex{}
	w.isClose = false
	client, err := NewClient(w.conf, func(msg []byte) {
		if res, ok := isResWsFunc(msg); ok {
			w.back <- res
		} else if res, ok := isCallWsFunc(msg); ok {
			w.call <- res
		}
	}, func(ws *WSClient, err error) {
		if w.client.isClose() == false {
			w.disconnect()
			for _, method := range w.disMethod {
				method(w)
			}
		}
	})
	if err != nil {
		close(w.back)
		close(w.call)
		close(w.callClose)
		w.client.Close()
		return nil, err
	}
	w.client = client
	go w.backFunc()
	return w, nil
}

func (w *WSRpcClient) backFunc() {
	for {
		select {
		case res := <-w.call:
			//远程调用客户端Func
			out, err := w.waiter[res.Waiter].RunClientMethod(res.Method, res.In)
			m, err := createResultData(res, out, err)
			if err != nil {
				break
			}
			err = w.client.SendMessage(m)
			if err != nil {
				log.Println(err)
			}
		case <-w.callClose:
			close(w.callClose)
			close(w.call)
			return
		}
	}
}

func (w *WSRpcClient) Close() {
	if w.isClose == false {
		w.client.Close()
		close(w.back)
		w.callClose <- true
	}
	w.isClose = true
	w.err <- nil
}

func (w *WSRpcClient) disconnect() {
	w.client.Close()
	close(w.back)
	w.callClose <- true
	w.err <- errors.New("disconnect")
}

func (w *WSRpcClient) Wait() error {
	errStr := <-w.err
	close(w.err)
	return errStr
}

func (w *WSRpcClient) CallFunc(waiter, method string, in map[string]interface{}) (map[string]interface{}, error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	rand := getRandString(4)
	res, err := createCallData(waiter, method, rand, in)
	if err != nil {
		return nil, err
	}
	err = w.client.SendMessage(res)
	if err != nil {
		return nil, err
	}
	t := time.NewTicker(time.Duration(TimeOut) * time.Second)
	defer t.Stop()
	for {
		select {
		case callback := <-w.back:
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
