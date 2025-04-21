package mid

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
)

// 代理管理者结构
type proxyManCol struct {
	leftConnMap  sync.Map //左侧（请求代理）服务端连接
	rightConnMap sync.Map //右侧（响应代理）服务端连接
	ctx          context.Context
	cancel       context.CancelFunc
}

func newProxyMan() *proxyManCol {
	ctx, cancel := context.WithCancel(context.Background())
	proxyMan = &proxyManCol{
		leftConnMap:  sync.Map{},
		rightConnMap: sync.Map{},
		ctx:          ctx,
		cancel:       cancel,
	}
	return proxyMan
}

func (p *proxyManCol) addLeftConn(conn *leftConn) {
	p.leftConnMap.Store(conn.UKey, conn)
}

func (p *proxyManCol) delLeftConn(uKey string) {
	p.leftConnMap.Delete(uKey)
}

func (p *proxyManCol) getLeftConn(uKey string) (*leftConn, error) {
	tmp, ok := p.leftConnMap.Load(uKey)
	if !ok {
		return nil, fmt.Errorf("left conn not found")
	}
	lConn, ok := tmp.(*leftConn)
	if !ok {
		return nil, fmt.Errorf("left conn is not leftConn")
	}
	return lConn, nil
}

func (p *proxyManCol) addRightConn(conn *rightConn) {
	if conn.Tag == "" {
		return
	}
	workers := make([]*rightConn, 0)
	if list, ok := p.rightConnMap.Load(conn.Tag); ok {
		if tmpList, ok1 := list.([]*rightConn); ok1 {
			workers = tmpList
		}
	}
	workers = append(workers, conn)
	p.rightConnMap.Store(conn.Tag, workers)
}

func (p *proxyManCol) getRightConn(tag string, strategyArr ...int) (*rightConn, error) {
	list, ok := p.rightConnMap.Load(tag)
	if !ok {
		return nil, fmt.Errorf("target proxy tag %s not found", tag)
	}
	workers, wOk := list.([]*rightConn)
	if !wOk {
		return nil, fmt.Errorf("target proxy tag %s is not a worker", tag)
	}
	wLen := len(workers)

	activeWorkers := make([]*rightConn, 0)
	for _, worker := range workers {
		if !worker.IsClose {
			activeWorkers = append(activeWorkers, worker)
		}
	}
	aLen := len(activeWorkers)

	if wLen != aLen {
		p.rightConnMap.Store(tag, activeWorkers)
	}

	if aLen < 1 {
		return nil, fmt.Errorf("没有可用的右侧服务：%s", tag)
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
