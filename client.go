package ws_rpc

import (
	"errors"
	"github.com/gorilla/websocket"
	"log"
	"time"
)

type OnCloseFunc func(ws *WSClient, err error)

type WSClient struct {
	conn      *websocket.Conn
	conf      ClientConf
	method    func(message []byte)
	onClose   OnCloseFunc
	writing   bool
	writeChan chan bool
	close     bool
}

//客户端配置
type ClientConf struct {
	Host   string
	Path   string
	Ticker int
}

//新建客户端:配置,回调函数,断开连接时调用函数
func NewClient(conf ClientConf, callback func(message []byte), onClose OnCloseFunc) (WSClient, error) {
	var client WSClient
	client.conf = conf
	client.method = callback
	client.onClose = onClose
	err := client.start()
	return client, err
}

func (ws *WSClient) SendMessage(data []byte) error {
	defer func() {
		recover()
	}()
	var err error
	if ws.close == false && ws.conn == nil {
		err = ws.start()
	}
	if err != nil {
		return err
	}
	if ws.conn == nil {
		return errors.New("client is close")
	}
	ws.writing = true
	err = ws.writeMessage(data)
	ws.writing = false
	return err
}

func (ws *WSClient) writeMessage(data []byte) error {
	ws.writeChan <- true
	err := ws.conn.WriteMessage(websocket.TextMessage, data)
	<-ws.writeChan
	return err
}

func (ws *WSClient) Close() {
	ws.close = true
	if ws.conn != nil {
		ws.conn.Close()
	}
	ws.conn = nil
}

func (ws *WSClient) start() error {
	ws.close = false
	ws.writing = false
	ws.writeChan = make(chan bool, 1)
	if ws.conn != nil {
		ws.conn.Close()
	}
	urls := "ws://" + ws.conf.Host + ws.conf.Path
	c, _, err := websocket.DefaultDialer.Dial(urls, nil)
	if err != nil {
		return err
	}
	ws.conn = c
	//读取信息
	go func() {
		for {
			_, m, err := ws.conn.ReadMessage()
			if err != nil {
				if ws.onClose != nil {
					ws.onClose(ws, err)
				}
				return
			}
			if !isPing(m) {
				ws.method(m)
			}
		}
	}()
	go func() {
		err := ws.ping(ws.conf.Ticker)
		if err != nil {
			ws.conn.Close()
		}
	}()
	return nil
}

func (ws *WSClient) isClose() bool {
	return ws.close
}

func isPing(msg []byte) bool {
	if string(msg) == BEAT {
		return true
	}
	return false
}

func (ws *WSClient) ping(ticket int) error {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()
	t := time.NewTicker(time.Duration(ticket) * time.Second)
	defer t.Stop()
	for {
		<-t.C
		if ws.isClose() == true {
			return nil
		}
		if !ws.writing {
			err := ws.writeMessage([]byte(BEAT))
			if err != nil {
				return err
			}
		}
	}
}
