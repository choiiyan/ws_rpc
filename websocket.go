package ws_rpc

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"log"
	"net/http"
	"sync"
)

type WS interface {
	OnClose(client *Client)
	OnMessage(client *Client, bytes []byte)
	OnConnect(client *Client)
}

//客户端 Client
type Client struct {
	//用户id
	id string
	//连接的socket
	socket *websocket.Conn
	//心跳
	beat bool
	//继承方法接口
	Context Context
	//方法接口
	method WS
	//组id
	group int64
	//用户信息
	userInfo map[string]interface{}
	//用户信息映射key
	userPrimary string
	//通道
	writeLock sync.Mutex
	//回调通道
	callChan chan *resultData
	callLock sync.Mutex
}

//创建客户端组管理器
var groupManager = sync.Map{}

//定义心跳消息
const BEAT = "@"

//客户端配置
type SeverConf struct {
	Port   int64
	Path   string
	Ticker int64
	method []MiddlewareFunc
}

type MiddlewareFunc func(c Context) error

func (s *SeverConf) MiddlewareFunc(method ...MiddlewareFunc) {
	s.method = append(s.method, method...)
}

//独立websocket服务
func Start(conf SeverConf, client WS) error {
	port := fmt.Sprint(conf.Port)
	if conf.Path == "" {
		conf.Path = "/"
	}
	New(conf.Ticker)
	http.HandleFunc(conf.Path, func(w http.ResponseWriter, r *http.Request) {
		context := NewContext(w, r)
		for _, m := range conf.method {
			err := m(context)
			if err != nil {
				return
			}
		}
		err := WSStart(context, client)
		if err != nil {
			log.Fatalln("update websocket err:", err)
		}
	})
	log.Println("WebSocket Server  port:" + port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalln("ListenAndServe:", err)
		return err
	}
	log.Println("WebSocket Server End")
	return nil
}

/*************方法用消息发送***************/

//发送消息到client对象
func SendMsgToClient(to *Client, msg []byte) {
	if to != nil {
		to.SendMsg(msg)
	}
}

//发送消息到uid
func SendMsgToUid(uid int64, msg []byte) {
	if client, ok := uidToClient.Load(uid); ok {
		client.(*Client).SendMsg(msg)
	}
}

//群发组消息
func SendMsgToGroup(gid int64, msg []byte) {
	if group, ok := groupManager.Load(gid); ok {
		//数据遍历成员
		group.(*sync.Map).Range(func(k, v interface{}) bool {
			if k.(string) != "len" {
				v.(*Client).SendMsg(msg)
			}
			return true
		})
	}
}

//发送消息给所以用户
func SendMsgToAll(msg []byte) {
	go func() {
		manager.clients.Range(func(k, v interface{}) bool {
			if conn, ok := v.(*Client); ok {
				conn.SendMsg(msg)
			}
			return true
		})
	}()
}

//获取连接数
func GetOnline() int64 {
	return manager.online
}

//获取组成员数量
func GetGroupUserCount(gid int64) int64 {
	count := int64(0)
	if group, ok := groupManager.Load(gid); ok {
		if groupLen, ok := group.(*sync.Map).Load("len"); ok {
			//数据遍历,成员退出组
			count = groupLen.(int64)
		}
	}
	return count
}

/******************************************/

//uid与client映射map
var uidToClient sync.Map

//uid与client绑定
func (c *Client) BindUidClient(uid interface{}) {
	if c == nil {
		return
	}
	uidToClient.Store(uid, c)
}

//uid与client解除绑定
func (c *Client) UnBindUidClient(uid interface{}) {
	uidToClient.Delete(uid)
}

//获取uid的client对象
func (c *Client) GetUidClient(uid interface{}) *Client {
	if client, ok := uidToClient.Load(uid); ok {
		return client.(*Client)
	}
	return nil
}

//保存用户信息
func (c *Client) SetUserInfo(user map[string]interface{}, primary string) {
	if c == nil {
		return
	}
	c.userInfo = user
	if primary != "" {
		c.userPrimary = primary
		if u, ok := user[primary]; ok {
			c.BindUidClient(u)
		}
	}
}

//获取用户信息
func (c *Client) GetUserInfo() map[string]interface{} {
	if c == nil {
		return nil
	}
	return c.userInfo
}

//发送消息到client对象
func (c *Client) SendMsgToClient(to *Client, msg []byte) {
	to.SendMsg(msg)
}

//发送消息到uid
func (c *Client) SendMsgToUid(uid interface{}, msg []byte) {
	if c == nil {
		return
	}
	c.GetUidClient(uid).SendMsg(msg)
}

//发送消息给所以用户
func (c *Client) SendMsgToAll(msg []byte) {
	go func() {
		manager.clients.Range(func(k, v interface{}) bool {
			if conn, ok := v.(*Client); ok {
				conn.SendMsg(msg)
			}
			return true
		})
	}()
}

//发送消息到除加入组的用户
func (c *Client) SendMsgNoGroup(msg []byte) {
	go func() { //通过协程发送消息
		manager.clients.Range(func(k, v interface{}) bool {
			if conn, ok := v.(*Client); ok {
				if conn.group == 0 {
					conn.SendMsg(msg)
				}
			}
			return true
		})
	}()
}

//加入组
func (c *Client) AddGroup(gid int64) {
	if c == nil {
		return
	}
	c.group = gid
	if gid > 0 {
		if group, ok := groupManager.Load(gid); ok {
			group.(*sync.Map).Store(c.id, c)
			if count, ok := group.(*sync.Map).Load("len"); ok {
				group.(*sync.Map).Store("len", count.(int64)+1)
			}
			//groupManager.Store(gid, group)
		} else {
			g := &sync.Map{}
			g.Store(c.id, c)
			g.Store("len", int64(1))
			groupManager.Store(gid, g)
		}
	}
}

//退出组
func (c *Client) ExitGroup() {
	if c == nil {
		return
	}
	if c.group > 0 {
		if group, ok := groupManager.Load(c.group); ok {
			group.(*sync.Map).Delete(c.id)
			if count, ok := group.(*sync.Map).Load("len"); ok {
				if count.(int64) == 1 {
					//如果组员空了则删除该组
					c.RemoveGroup(c.group)
				} else {
					group.(*sync.Map).Store("len", count.(int64)-1)
				}
			}
		}
	}
	c.group = 0
}

//删除组
func (c *Client) RemoveGroup(gid int64) {
	if gid > 0 {
		if group, ok := groupManager.Load(gid); ok {
			//数据遍历,成员退出组
			group.(*sync.Map).Range(func(k, v interface{}) bool {
				if k.(string) != "len" {
					v.(*Client).group = 0
				}
				return true
			})
			groupManager.Delete(gid)
		}
	}
}

//获取id
func (c *Client) GetId() string {
	return c.id
}

//获取组id
func (c *Client) GetGId() int64 {
	return c.group
}

//返回文本消息
func (c *Client) Text(i interface{}) []byte {
	return []byte(fmt.Sprint(i))
}

//返回json格式
func (c *Client) JSON(i interface{}) []byte {
	enc, err := json.Marshal(i)
	if err != nil {
		return []byte("")
	}
	return enc
}

//发送消息
func (c *Client) SendMsg(msg []byte) {
	if c == nil {
		return
	}
	c.write(msg)
}

//群发组消息
func (c *Client) SendMsgToGroup(msg []byte) {
	if c.group > 0 {
		if group, ok := groupManager.Load(c.group); ok {
			//数据遍历成员
			group.(*sync.Map).Range(func(k, v interface{}) bool {
				if k.(string) != "len" {
					v.(*Client).SendMsg(msg)
				}
				return true
			})
		}
	}
}

//群发组消息除自己
func (c *Client) SendMsgToGroupNoOwn(msg []byte) {
	if c == nil {
		return
	}
	if c.group > 0 {
		if group, ok := groupManager.Load(c.group); ok {
			//数据遍历,成员退出组
			group.(*sync.Map).Range(func(k, v interface{}) bool {
				if k.(string) != c.id && k.(string) != "len" {
					v.(*Client).SendMsg(msg)
				}
				return true
			})
		}
	}
}

//群发消息
func (c *Client) BroadcastMsg(msg []byte) {
	//遍历已经连接的客户端，把消息发送给他们
	manager.clients.Range(func(k, v interface{}) bool {
		if conn, ok := v.(*Client); ok {
			conn.write(msg)
		}
		return true
	})
}

//群发消息,排除自己
func (c *Client) BroadcastMsgNoOwn(msg []byte) {
	if c == nil {
		return
	}
	//遍历已经连接的客户端，把消息发送给他们
	manager.clients.Range(func(k, v interface{}) bool {
		if conn, ok := v.(*Client); ok {
			//不给自己
			if conn != c {
				conn.write(msg)
			}
		}
		return true
	})
}

//设置允许跨域
var upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

//解析WS连接
func WSStart(c Context, w WS) error {
	//解析ws连接
	conn, err := upgrade.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	uid, _ := uuid.NewV4()
	//初始化一个客户端对象
	client := &Client{
		id:        uid.String(),
		socket:    conn,
		beat:      true,
		Context:   c,
		method:    w,
		writeLock: sync.Mutex{},
		callChan:  make(chan *resultData),
		callLock:  sync.Mutex{},
	}
	//把这个对象发送给 管道
	manager.register <- client
	//创建处理协程
	go client.read()
	return nil
}

//创建连接
func (c *Client) onConnect() {
	c.method.OnConnect(c)
}

//收到信息时处理
func (c *Client) onMessage(msg []byte) {
	if isHeartbeat(msg) {
		c.SendMsg(msg)
		return
	}
	c.method.OnMessage(c, msg)
}

//判断是否为心跳
func isHeartbeat(msg []byte) bool {
	if string(msg) == BEAT {
		return true
	}
	return false
}

//断开连接
func (c *Client) onClose() {
	c.method.OnClose(c)
	close(c.callChan)
}

//关闭连接
func (c *Client) Close() {
	err := c.socket.Close()
	if err == nil {
		c.onClose()
	}
}

//定义客户端结构体的read方法
func (c *Client) read() {
	defer func() {
		manager.unregister <- c
		c.Close()
		recover()
	}()
	c.onConnect() //调用创建时处理方法
	for {
		//读取消息
		_, message, err := c.socket.ReadMessage()
		//如果有错误信息，就注销这个连接然后关闭
		if err != nil {
			manager.unregister <- c
			c.Close()
			return
		}
		//收到信息发送到处理方法
		c.beat = true
		go c.onMessage(message)
	}
}

//写入管道后激活这个进程
func (c *Client) write(message []byte) {
	c.writeLock.Lock()
	defer func() {
		c.writeLock.Unlock()
		recover()
	}()
	//有消息就写入，发送给web端
	err := c.socket.WriteMessage(websocket.TextMessage, message)
	//写不成功数据就关闭
	if err != nil {
		manager.unregister <- c
		c.Close()
		return
	}
}
