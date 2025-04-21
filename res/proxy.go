package res

import (
	"context"
	"encoding/json"
	"fmt"
	"hz_proxy/config"
	"hz_proxy/utils"
	"log"
	"math/rand"
	"net"
	"sort"
	"sync"
	"time"
)

type proxyManCol struct {
	targetConnMap sync.Map `comment:"中间服务端连接  map[con_u_key] = conn"`
	midConnMap    sync.Map `comment:"中间服务端连接  map[con_u_key] = conn"`
	ctx           context.Context
	cancel        context.CancelFunc
}

func newProxyMan() *proxyManCol {
	ctx, cancel := context.WithCancel(context.Background())
	proxyMan = &proxyManCol{
		targetConnMap: sync.Map{},
		midConnMap:    sync.Map{},
		ctx:           ctx,
		cancel:        cancel,
	}
	return proxyMan
}

func (p *proxyManCol) addMidConn(conn *midConn) {
	if conn.UKey == "" {
		return
	}
	p.midConnMap.Store(conn.UKey, conn)
}

func (p *proxyManCol) delMidConn(uKey string) {
	p.midConnMap.Delete(uKey)
}

func (p *proxyManCol) addTargetConn(conn *targetConn) {
	p.targetConnMap.Store(conn.UKey, conn)
}

func (p *proxyManCol) delTargetConn(uKey string) {
	p.targetConnMap.Delete(uKey)
}

func (p *proxyManCol) getTargetConn(uKey string) (*targetConn, error) {
	tmp, tmpOk := p.targetConnMap.Load(uKey)
	if !tmpOk {
		return nil, fmt.Errorf("未找到目标服务连接")
	}
	ret, ok := tmp.(*targetConn)
	if !ok {
		return nil, fmt.Errorf("未找到目标服务连接")
	}
	return ret, nil
}

func (p *proxyManCol) getMidConn(strategyArr ...int) (*midConn, error) {

	workers := make([]*midConn, 0)
	p.midConnMap.Range(func(key, value interface{}) bool {
		if tmp, ok := value.(*midConn); ok {
			workers = append(workers, tmp)
		}
		return true
	})

	activeWorkers := make([]*midConn, 0)
	for _, worker := range workers {
		if worker.IsClose {
			p.midConnMap.Delete(worker.UKey)
			continue
		}
		activeWorkers = append(activeWorkers, worker)
	}
	aLen := len(activeWorkers)

	if aLen < 1 {
		return nil, fmt.Errorf("没有可用的mid服务")
	}

	strategy := 0
	if len(strategyArr) > 0 {
		strategy = strategyArr[0]
	}

	tIndex := 0
	switch strategy {
	case 1:
		//随机
		tIndex = rand.Intn(aLen)
		//todo 策略
	}

	return workers[tIndex], nil
}

func (p *proxyManCol) insureConnects() {
	ctx, cancel := context.WithCancel(context.Background())
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		cancel()
		ticker.Stop()
	}()
	for {
		select {
		case <-ticker.C:
			p.checkConnects()
		case <-ctx.Done():
			return
		}
	}
}

func (p *proxyManCol) initConnects(poolSize int) {
	conf := config.Conf.Res
	for i := 0; i < poolSize; i++ {
		if err := p.oneConnect(); err != nil {
			log.Println(err)
		} else {
			fmt.Printf("%s_%d连接成功\r\n", conf.Tag, i)
		}
	}
}

func (p *proxyManCol) oneConnect() error {
	cFunc := func(retChan chan string) {
		conf := config.Conf.Res

		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", conf.MidHost, conf.MidPort))
		if err != nil {
			retChan <- err.Error()
			return
		}
		tmpConn, _ := utils.NewFdConn(conn, conf.IsEncrypt, conf.EncryptMethod, conf.EncryptToken)
		sConn := &midConn{
			FdConn: tmpConn,
		}

		defer func() {
			if err1 := sConn.close(); err1 != nil {
				fmt.Println(err1)
			}
		}()

		//认证：发送用户名和密码
		tmpMsg := utils.StructMsg{Username: conf.Username, Password: conf.Pwd, ProxyTag: conf.Tag}
		sendMsg, _ := json.Marshal(tmpMsg)
		if err = sConn.Write(sendMsg, []byte{}, []byte(utils.EventAuth)); err != nil {
			retChan <- err.Error()
			return
		}
		authMsg := utils.MsgDecode(sConn.Conn)
		if authMsg.Err != nil {
			retChan <- authMsg.Err.Error()
		}
		if string(authMsg.Msg) != "ok" {
			retChan <- "认证失败"
			return
		}
		//连接建立成功
		retChan <- "ok"

		proxyMan.addMidConn(sConn)

		//接收来自中间服务器的消息
		for {
			select {
			case <-sConn.Ctx.Done():
				return
			default:
				tMsg := utils.MsgDecode(sConn.Conn)
				if tMsg.Err != nil {
					log.Println(tMsg.Err)
					return
				}
				if tMsg.ActionString() == utils.EventShake {
					go func() {
						<-dailTarget(sConn, tMsg)
					}()
				} else {
					tConn, tErr := proxyMan.getTargetConn(tMsg.StreamIdString())
					if tErr == nil {
						tConn.Conn.Write(tMsg.Msg)
					}
				}
			}
		}

	}

	tmpChan := make(chan string)
	go cFunc(tmpChan)

	if tmpRet := <-tmpChan; tmpRet != "ok" {
		return fmt.Errorf(tmpRet)
	}
	return nil
}

func (p *proxyManCol) checkConnects() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	list := make([]*midConn, 0)
	p.midConnMap.Range(func(key, value interface{}) bool {
		conn, ok := value.(*midConn)
		if !ok {
			p.delMidConn(key.(string))
			return true
		}
		if conn.IsClose {
			p.delMidConn(key.(string))
			return true
		}
		list = append(list, conn)
		return true
	})
	listLen := len(list)
	if listLen > config.Conf.Res.PoolSize {
		sort.Slice(list, func(i, j int) bool {
			//根据时间倒序
			return list[i].STime.After(list[j].STime)
		})
		//需要删除
		for i := 0; i < listLen-config.Conf.Res.PoolSize-listLen; i++ {
			if err := list[i].close(); err != nil {
				log.Println("checkConnects删除连接失败：", err.Error(), "，连接id：", list[i].UKey, "，连接类型：", "mid")
			}
		}
		return
	}
	//需要创建
	if config.Conf.Res.PoolSize > listLen {
		p.initConnects(config.Conf.Res.PoolSize - listLen)
	}

}
