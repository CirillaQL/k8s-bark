package bark

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	BARK_SERVER_OK  = 0
	BARK_SERVER_ERR = 1
)

type Bark struct {
	barkServer string
	status     int
}

// NewBark 创建一个新的bark实例
func NewBark(barkServer string) *Bark {
	return &Bark{
		barkServer: barkServer,
		status:     BARK_SERVER_OK,
	}
}

// HealthzCheck 检查bark服务器是否可用
func (b *Bark) HealthzCheck() {
	for {
		resp, err := http.Get("http://" + b.barkServer + "/healthz")
		if err != nil {
			panic(err)
		}
		s, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		status := string(s)
		if status == "ok" {
			b.status = BARK_SERVER_OK
			fmt.Println("bark server is ok")
		} else {
			b.status = BARK_SERVER_ERR
			fmt.Println("bark server is not ok")
		}
		time.Sleep(5 * time.Second)
	}
}
