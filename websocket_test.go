package ws_rpc

import (
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"qingliao/extension/tools"
	"sync"
	"testing"
	"time"
)

type ws struct {
}

func TestServer(t *testing.T) {
	conf := SeverConf{
		Port:   38888,
		Path:   "/",
		Ticker: 5,
	}
	secret := "MyDarkSecret"
	hashLoop := tools.NewHashLoop(1000)
	conf.MiddlewareFunc(func(c Context) error {
		token := c.GetFrom()["token"]
		err := bcrypt.CompareHashAndPassword([]byte(token), []byte(secret))
		if err != nil {
			return err
		}
		if !hashLoop.Loop(token) {
			return errors.New("key has been used")
		}
		return nil
	})
	//启动服务
	err := Start(conf, &ws{})
	fmt.Println(err)
}

func ShowIp(c Context) error {
	c.Set("key", c.Request().RemoteAddr)
	return nil
}

func Secret(c Context) error {
	password := []byte("MyDarkSecret")
	// Comparing the password with the hash
	err := bcrypt.CompareHashAndPassword([]byte(c.GetFrom()["token"]), password)
	fmt.Println(err) //
	return nil
}

//创建连接
func (ws *ws) OnConnect(client *Client) {
}

//收到信息时处理
func (ws *ws) OnMessage(client *Client, msg []byte) {
	client.SendMsg(msg)
}

//断开连接
func (ws *ws) OnClose(client *Client) {
}

func TestServer2(t *testing.T) {
	conf := SeverConf{
		Port:   38888,
		Path:   "/",
		Ticker: 5,
	}
	secret := "MyDarkSecret"
	hashLoop := tools.NewHashLoop(1000)
	conf.MiddlewareFunc(func(c Context) error {
		token := c.GetFrom()["token"]
		err := bcrypt.CompareHashAndPassword([]byte(token), []byte(secret))
		if err != nil {
			return err
		}
		if !hashLoop.Loop(token) {
			return errors.New("key has been used")
		}
		return nil
	})
	//启动服务
	err := Start(conf, &ws{})
	fmt.Println(err)
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
			fmt.Println(err)
		}
	}()
	password := []byte("MyDarkSecret")
	hashedPassword, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return
	}

	conf := ClientConf{
		Host:   "127.0.0.1:38888",
		Path:   "/?token=" + string(hashedPassword),
		Ticker: 5,
	}
	client, err := NewClient(conf, callback,nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	go func() {
		time.Sleep(time.Duration(60*10) * time.Second)
		client.Close()
	}()
	ticket := time.NewTicker(time.Duration(1) * time.Second)
	defer ticket.Stop()
	for {
		<-ticket.C
		err := client.SendMessage([]byte("sss"))
		if err != nil {
			//fmt.Println("err:", err)
			return
		}
	}
}

func callback(msg []byte) {
	//fmt.Println(string(msg))
}
