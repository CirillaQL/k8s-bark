package bark

import (
	"encoding/json"
	"io/ioutil"
	"k8s-bark/pkg/log"
	"net/http"
	"time"
)

var LOG = log.LOG

const (
	BARK_SERVER_OK  = 0
	BARK_SERVER_ERR = 1
)

type Bark struct {
	barkServer  string
	barkToken   string
	status      int
	messageChan chan Message
}

// NewBark 创建一个新的bark实例
func NewBark(barkServer, barkToken string) *Bark {
	return &Bark{
		barkServer:  barkServer,
		barkToken:   barkToken,
		status:      BARK_SERVER_OK,
		messageChan: make(chan Message, 10),
	}
}

// HealthzCheck 检查bark服务器是否可用
func (b *Bark) HealthzCheck() {
	for {
		resp, err := http.Get("http://" + b.barkServer + "/healthz")
		if err != nil {
			LOG.Errorf("bark server %s is not available, Error: %s", b.barkServer, err.Error())
		} else {
			_json := HealthResponse{}
			err = json.NewDecoder(resp.Body).Decode(&_json)
			if err != nil {
				LOG.Errorf("bark server %s is not available", b.barkServer)
				panic(err)
			}
			status := _json.Code
			message := _json.Message
			LOG.Info(status)
			LOG.Info(message)
			resp.Body.Close()
		}
		time.Sleep(5 * time.Second)
	}
}

// Push 向bark的Channel写入消息
func (b *Bark) Push(message Message) {
	b.messageChan <- message
}

// Send 向bark发送消息
func (b *Bark) Send() {
	for {
		message := <-b.messageChan
		if b.status != BARK_SERVER_OK {
			continue
		}
		resp, err := http.Get("http://" + b.barkServer + "/" + b.barkToken + "/" + message.Status + "/" + message.Information)
		if err != nil {
			LOG.Errorf("bark server %s is not available, Error: %s", b.barkServer, err.Error())
		} else {
			s, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				LOG.Errorf("bark server %s is not available", b.barkServer)
				panic(err)
			}
			status := string(s)
			if status == "ok" {
				b.status = BARK_SERVER_OK
			} else {
				b.status = BARK_SERVER_ERR
				LOG.Warnf("bark server %s is error, status is %s", b.barkServer, status)
			}
		}
	}
}
