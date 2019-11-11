package ws_rpc

import (
	"net/http"
	"sync"
)

type Context interface {
	Response() http.ResponseWriter
	Request() *http.Request
	Get(key string) interface{}
	Set(key string, val interface{})
	GetFrom() map[string]string
}
type context struct {
	response http.ResponseWriter
	request  *http.Request
	other    sync.Map
	GET      map[string]string
}

func NewContext(w http.ResponseWriter, r *http.Request) Context {
	val := r.URL.Query()
	get := make(map[string]string)
	for k, v := range val {
		get[k] = v[0]
	}
	return &context{
		response: w,
		request:  r,
		GET:      get,
	}
}

func (c *context) GetFrom() map[string]string {
	return c.GET
}

func (c *context) Request() *http.Request {
	return c.request
}

func (c *context) Response() http.ResponseWriter {
	return c.response
}

func (c *context) Get(key string) interface{} {
	res, _ := c.other.Load(key)
	return res
}

func (c *context) Set(key string, val interface{}) {
	c.other.Store(key, val)
}
