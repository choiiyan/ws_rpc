package ws_rpc

import (
	"sync"
	"time"
)

//客户端管理
type ClientManager struct {
	//客户端组管理
	groupManager sync.Map
	//客户端 map 储存并管理所有的长连接client
	clients sync.Map
	//clients map[string]*Client
	//新创建的长连接client
	register chan *Client
	//新注销的长连接client
	unregister chan *Client
	//在线连接数
	online int64
}

//New WS管理器
func NewManager(t int64) *ClientManager {
	manager := new(ClientManager)
	manager.register = make(chan *Client, 1)
	manager.unregister = make(chan *Client, 1)
	manager.clients = sync.Map{}
	manager.groupManager = sync.Map{}
	manager.online = 0
	go manager.start()
	if t > 0 {
		go manager.beat(t) //开启心跳检测
	}
	return manager
}

//心跳检测 每秒遍历一次
func (manager *ClientManager) beat(t int64) {
	ticker := time.NewTicker(time.Duration(t+1) * time.Second)
	defer ticker.Stop()
	for {
		<-ticker.C
		//数据遍历
		unregister := make([]*Client, 0)
		manager.clients.Range(func(k, v interface{}) bool {
			if conn, ok := v.(*Client); ok {
				if conn.beat == false {
					unregister = append(unregister, conn)
					conn.Close()
				}
				conn.beat = false
			}
			return true
		})
		for _, conn := range unregister {
			manager.unregister <- conn
		}
	}
}

func (manager *ClientManager) start() {
	for {
		select {
		//如果有新的连接接入,就通过channel把连接传递给conn
		case conn := <-manager.register:
			//储存客户端的连接
			manager.clients.Store(conn.id, conn)
			manager.online++
			//如果连接断开了
		case conn := <-manager.unregister:
			//判断连接的状态，如果是true,就关闭send，删除连接client的值
			if _, ok := manager.clients.Load(conn.id); ok {
				if conn.group > 0 {
					conn.ExitGroup()
				}
				manager.clients.Delete(conn.id)
				if manager.online > 0 {
					//判断是佛删除成员成功
					if _, ok := manager.clients.Load(conn.id); !ok {
						manager.online--
					} else {
						manager.unregister <- conn
					}
				}
				pri := conn.userPrimary
				if pri != "" {
					if _, ok := conn.userInfo[pri]; ok { //删除用户与client的映射
						uidToClient.Delete(conn.userInfo[pri])
					}
				}
			}
		}
	}
}
