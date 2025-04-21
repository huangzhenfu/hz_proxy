package mid

import (
	"fmt"
	"log"
)

// 代理管理者
var proxyMan = &proxyManCol{}

func Run() {
	proxyMan = newProxyMan()
	go func() {
		if err := ReqServer(); err != nil {
			log.Fatal(fmt.Errorf("启动左侧服务失败：: %v", err))
		}
	}()
	if err := ResServer(); err != nil {
		log.Fatal(fmt.Errorf("启动右侧服务失败：: %v", err))
	}
}
