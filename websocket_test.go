package ws_rpc

import (
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"log"
	"strconv"
	"sync"
	"testing"
	"time"
)

type ws struct {
}

//创建连接
func (ws *ws) OnConnect(client *Client) {
}

//收到信息时处理
func (ws *ws) OnMessage(client *Client, msg []byte) {
	log.Println(string(msg))
	client.SendMsg(msg)
}

//断开连接
func (ws *ws) OnClose(client *Client) {
}

//WS管理
var WSManager *ClientManager

//WS服务器
func TestServer(t *testing.T) {
	conf := SeverConf{
		Port:   48888,
		Path:   "/",
		Ticker: 5,
	}
	log.Println("server start")
	ws := NewWsServer(conf, &ws{})
	//管理
	WSManager = ws.Manager
	ws.MiddlewareFunc(VerifyToken)
	err := ws.Start()
	fmt.Println(err)
}

var hashLoop = NewHashLoop(1000)
func VerifyToken(c Context) error {
	secret := "MyDarkSecret"
	token := c.GetFrom()["token"]
	err := bcrypt.CompareHashAndPassword([]byte(token), []byte(secret))
	if err != nil {
		return err
	}
	if !hashLoop.Loop(token) {
		return errors.New("key has been used")
	}
	return nil
}

func TestClient(t *testing.T) {
	wg := sync.WaitGroup{}
	for i := 0; i < 1; i++ {
		time.Sleep(time.Microsecond * time.Duration(1))
		wg.Add(1)
		go func() {
			startClient()
			wg.Done()
		}()
	}
	wg.Wait()
}

func startClient() {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()
	password := []byte("MyDarkSecret")
	hashedPassword, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return
	}

	conf := ClientConf{
		Host:   "127.0.0.1:48888",
		Path:   "/?token=" + string(hashedPassword),
		Ticker: 5,
	}
	client, err := NewClient(conf, callback, nil)
	if err != nil {
		log.Println(err)
		return
	}
	go func() {
		time.Sleep(time.Duration(60*10) * time.Second)
		client.Close()
	}()
	ticket := time.NewTicker(time.Duration(50) * time.Microsecond)
	i := 0
	defer ticket.Stop()
	for {
		<-ticket.C
		i++
		msg := "test " + strconv.Itoa(i)
		err := client.SendMessage([]byte(msg))
		if err != nil {
			log.Println("err:", err)
			return
		}
	}
}

func callback(msg []byte) {
	fmt.Println(string(msg))
}
