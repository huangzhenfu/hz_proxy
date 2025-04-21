package res

import (
	"encoding/json"
	"fmt"
	"hz_proxy/config"
	log2 "hz_proxy/log"
	"hz_proxy/utils"
	"net"
)

var proxyMan *proxyManCol

type midConn struct {
	*utils.FdConn
}

func Run() {
	proxyMan = newProxyMan()
	conf := config.Conf.Res
	proxyMan.initConnects(conf.PoolSize)
	proxyMan.insureConnects()
}

func (s *midConn) close() error {
	proxyMan.delMidConn(s.UKey)
	if err := s.FdConn.Close(); err != nil {
		return err
	}
	return nil
}

type targetConn struct {
	*utils.FdConn
}

func (s *targetConn) close() error {
	proxyMan.delTargetConn(s.UKey)
	if err := s.FdConn.Close(); err != nil {
		return err
	}
	return nil
}

func dailTarget(midCon *midConn, msg utils.MsgCol) chan string {
	retChan := make(chan string)

	cFunc := func(midCon *midConn, msg utils.MsgCol, v chan string) {

		streamId := msg.StreamIdString()

		targetInfo := &utils.StructMsg{}
		if err := json.Unmarshal(msg.Msg, targetInfo); err != nil {
			retChan <- fmt.Sprintf("句柄：%s,请求握手失败，解析目标服务失败：%v", streamId, err.Error())
			return
		}

		tConn, err := net.Dial("tcp", targetInfo.TargetAddr)
		if err != nil {
			retChan <- fmt.Sprintf("句柄：%s,目标服务：%s，请求握手失败：%v", streamId, targetInfo.TargetAddr, err.Error())
			return
		}
		fdConn, _ := utils.NewFdConn(tConn, false, "", "")
		sConn := &targetConn{
			FdConn: fdConn,
		}
		defer func() {
			if err1 := sConn.close(); err1 != nil {
				fmt.Println(err1)
			}
		}()
		sConn.UKey = msg.StreamIdString()
		proxyMan.addTargetConn(sConn)

		//连接建立成功，发送握手成功消息
		v <- "ok"
		midCon.Write([]byte("ok"), msg.StreamId, []byte(utils.EventShake))
		log2.Debug(fmt.Sprintf("句柄：%s,目标服务：%s，握手成功", streamId, targetInfo.TargetAddr))

		//成功建立连接后，监听目标服务器发过来的数据
		for {
			select {
			case <-sConn.Ctx.Done():
				log2.Debug(fmt.Sprintf("句柄：%s，目标服务：%s，目标服务断开连接：%v", streamId, targetInfo.TargetAddr, "ctx断开"))
				return
			default:
				buf := make([]byte, 4096)
				msgLen, msgErr := sConn.Reader.Read(buf)
				if msgErr != nil {
					//目标服务器断开
					midCon.Write([]byte("close"), msg.StreamId, []byte(utils.EventClose))
					log2.Debug(fmt.Sprintf("句柄：%s，目标服务：%s，目标服务断开连接：%v", streamId, targetInfo.TargetAddr, msgErr.Error()))
					return
				}
				midCon.Write(buf[:msgLen], msg.StreamId, []byte(utils.EventTransfer))
				log2.Debug(fmt.Sprintf("句柄：%s，目标服务：%s，目标服务发送消息：%v字节", streamId, targetInfo.TargetAddr, msgLen))
			}
		}
	}

	go cFunc(midCon, msg, retChan)

	return retChan
}
