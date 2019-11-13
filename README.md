# ws-rpc
基于websocket的双向通讯rpc模块



##### 服务端:

```go

func TestNewWsRpcServer(t *testing.T) {
	secret := "MyDarkSecret"					 //连接密匙
	server := NewWsRpcServer(38888, secret)
	server.RegisterWaiter("test", &ClientTest{}) //注册服务(可多次注册)
	//server.OnConnectFunc(callbackFunc)		 //客户端连接时调用
	server.OnCloseFunc(callbackFunc)			 //客户端断开时调用
	err := server.Start()
	log.Println(err)
}

func callbackFunc(c *Client) {
	log.Println(c.Context.Request().RemoteAddr)
}

type ClientTest struct{}

func (t *ClientTest) Test(c *Client, in map[string]interface{}) (map[string]interface{}, error) {
    //客户端注册的方法 目标客户端,服务,方法,参数    return:(map[string]interface{}, error)
	data, err := CallClientFunc(c, "test", "test", in)  //广播时使用协程可提高效率
	log.Println("CallClientFunc: ", data, err)
	return in, nil
}
```



##### 客户端:

```go
func TestNewWsRpcClient(t *testing.T) {
	secret := "MyDarkSecret"							//连接密匙
	client, err := NewWsRpcClient("127.0.0.1:38888", secret).
		RegisterWaiter("test", &Callback{}). 			//注册服务(可多次注册)
		DisconnectFunc(Disconnect). 					//连接中断处理(可多次注册)
		Start()
	if err != nil {
		log.Println(err)
		return
	}
	//调用服务端注册的方法
	data, err := client.CallFunc("test", "test",
		map[string]interface{}{
			"in": "sss",
		})
	log.Println(data, err)
	log.Println("Done")
	client.Close() //结束连接
	err = client.Wait()
	if err != nil {
		log.Println(err)
	}
}

func Disconnect(w *WSRpcClient) {
	log.Println("Disconnect...")
}

type Callback struct{}

func (c *Callback) Test(in map[string]interface{}) (map[string]interface{}, error) {
	log.Println("Server Callback: ", in)
	return nil, nil
}
```

