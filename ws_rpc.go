package ws_rpc

import (
	"encoding/json"
	"errors"
	"math/rand"
	"reflect"
	"sync"
	"time"
)

const TimeOut = 3
const Ticker = 5

type callData struct {
	Waiter string                 `json:"a"`
	Method string                 `json:"b"`
	In     map[string]interface{} `json:"c"`
	Random string                 `json:"d"`
}

type resultData struct {
	Waiter string                 `json:"a"`
	Method string                 `json:"b"`
	Out    map[string]interface{} `json:"c"`
	Err    error                  `json:"d"`
	Random string                 `json:"e"`
}

func createCallData(waiter, method, rand string, data map[string]interface{}) ([]byte, error) {
	d := callData{
		Waiter: stringToLower(waiter),
		Method: stringToLower(method),
		In:     data,
		Random: "c" + rand,
	}
	return encoder(&d)
}

func createResultData(call *callData, data map[string]interface{}, err error) ([]byte, error) {
	d := resultData{
		Waiter: call.Waiter,
		Method: call.Method,
		Out:    data,
		Err:    err,
		Random: "r" + call.Random,
	}
	return encoder(&d)
}

func isResWsFunc(msg []byte) (*resultData, bool) {
	res := new(resultData)
	err := decoder(msg, res)
	if err != nil {
		return nil, false
	}
	if res.Method == "" || res.Random == "" || res.Random[:1] != "r" {
		return nil, false
	}
	res.Random = res.Random[1:]
	return res, true
}

func isCallWsFunc(msg []byte) (*callData, bool) {
	res := new(callData)
	err := decoder(msg, res)
	if err != nil {
		return nil, false
	}
	if res.Method == "" || res.Random == "" || res.Random[:1] != "c" {
		return nil, false
	}
	res.Random = res.Random[1:]
	return res, true
}

type Waiter struct {
	MethodMap map[string]reflect.Value
}

//运行方法
func (w *Waiter) RunMethod(name string, c interface{}, in interface{}) (map[string]interface{}, error) {
	if method, ok := w.MethodMap[stringToLower(name)]; ok {
		rel := method.Call([]reflect.Value{reflect.ValueOf(c), reflect.ValueOf(in)})
		if len(rel) != 2 {
			return nil, errors.New("类型错误,方式输出类型需为(map[string]interface{}, error)")
		}
		if _, ok := rel[0].Interface().(map[string]interface{}); !ok {
			return nil, errors.New("类型错误,方式输出类型需为(map[string]interface{}, error)")
		}
		var err error
		if rel[1].Interface() == nil {
			err = nil
		} else {
			err = rel[1].Interface().(error)
		}
		return rel[0].Interface().(map[string]interface{}), err
	}
	return nil, errors.New("找不到方法")
}

//运行客户端方法
func (w *Waiter) RunClientMethod(name string, in interface{}) (map[string]interface{}, error) {
	if method, ok := w.MethodMap[stringToLower(name)]; ok {
		rel := method.Call([]reflect.Value{reflect.ValueOf(in)})
		if len(rel) != 2 {
			return nil, errors.New("类型错误,方式输出类型需为(map[string]interface{}, error)")
		}
		if _, ok := rel[0].Interface().(map[string]interface{}); !ok {
			return nil, errors.New("类型错误,方式输出类型需为(map[string]interface{}, error)")
		}
		var err error
		if rel[1].Interface() == nil {
			err = nil
		} else {
			err = rel[1].Interface().(error)
		}
		return rel[0].Interface().(map[string]interface{}), err
	}
	return nil, errors.New("找不到方法")
}

//创建服务
func NewWaiter(obj interface{}) *Waiter {
	w := new(Waiter)
	objValue := reflect.ValueOf(obj)
	objType := reflect.TypeOf(obj)
	w.MethodMap = make(map[string]reflect.Value)
	if objType.Elem().Kind() != reflect.Struct {
		return nil
	}
	for i := 0; i < objType.NumMethod(); i += 1 {
		name := objType.Method(i).Name
		w.MethodMap[stringToLower(name)] = objValue.MethodByName(name)
	}
	return w
}

type HashLoop struct {
	size       int
	p          int
	hashMap    map[string]int //hash-->n
	refHashMap map[int]string //n-->hash
	look       sync.Mutex
}

//创建HashLoop
func NewHashLoop(size int) HashLoop {
	loop := HashLoop{
		size:       size - 1,
		p:          0,
		hashMap:    map[string]int{},
		refHashMap: map[int]string{},
		look:       sync.Mutex{},
	}
	return loop
}

func (h *HashLoop) Loop(key string) bool {
	h.look.Lock()
	defer h.look.Unlock()
	if _, ok := h.hashMap[key]; ok {
		return false
	}
	if _, ok := h.refHashMap[h.p]; ok {
		delete(h.hashMap, h.refHashMap[h.p])
	}
	h.hashMap[key] = h.p
	h.refHashMap[h.p] = key
	if h.p == h.size {
		h.p = 0
	} else {
		h.p++
	}
	return true
}

func hexLoop(h byte) byte {
	if h == 0xff {
		return 0x00
	}
	h++
	return h
}

func encoder(data interface{}) ([]byte, error) {
	rand.Seed((time.Now().UnixNano() + rand.Int63()) ^ rand.Int63())
	hex := byte(rand.Int() % 255)
	res, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	str := string(res)
	str = str[1 : len(str)-1]
	res = []byte(str)
	d := make([]byte, 0)
	d = append(d, hex)
	for _, v := range res {
		hex = hexLoop(hex)
		d = append(d, v^hex)
	}
	return d, nil
}

func decoder(data []byte, res interface{}) error {
	d := make([]byte, 0)
	hex := data[0]
	for k, v := range data {
		if k != 0 {
			hex = hexLoop(hex)
			d = append(d, v^hex)
		}
	}
	str := "{" + string(d) + "}"
	return json.Unmarshal([]byte(str), res)
}

//转小写 例:AbcAbc--->abc_abc
func stringToLower(str string) string {
	b := make([]byte, len(str))
	ii := 0
	for i, _ := range str {
		s := str[i]
		if 'A' <= s && s <= 'Z' {
			s = s - 'A' + 'a'
			if i != 0 {
				b[i] = '_'
				ii++
			}
			b[i] = s
		} else {
			b[i] = s
		}
		ii++
	}
	return string(b)
}

func getRandString(len int) string {
	rand.Seed((time.Now().UnixNano() + rand.Int63()) ^ rand.Int63())
	var randStr []byte
	for i := 0; i < len; i++ {
		switch rand.Int() % 3 {
		case 0:
			randStr = append(randStr, byte(rand.Int()%8)+'2')
		case 1:
			randStr = append(randStr, byte(rand.Int()%26)+'a')
		case 2:
			randStr = append(randStr, byte(rand.Int()%26)+'A')
		}
	}
	return string(randStr)
}


func jsonEncode(j interface{}) string {
	b, _ := json.Marshal(j)
	return string(b)
}

func jsonDecode(j string) map[string]interface{} {
	bak := make(map[string]interface{})
	err := json.Unmarshal([]byte(j), &bak)
	if err != nil {
		return nil
	}
	return bak
}