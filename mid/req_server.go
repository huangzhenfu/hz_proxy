package mid

import (
	"encoding/json"
	"fmt"
	"hz_proxy/config"
	"hz_proxy/log"
	"hz_proxy/utils"
	"net"
)

type leftConn struct {
	*utils.FdConn
}

func (s *leftConn) close() error {
	proxyMan.delLeftConn(s.UKey)
	if err := s.FdConn.Close(); err != nil {
		return err
	}
	return nil
}

func ReqServer() error {
	conf := config.Conf.MidReq
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", conf.Port))
	if err != nil {
		return err
	}
	defer func() {
		if err = listen.Close(); err != nil {
			fmt.Println(err)
		}
	}()
	for {
		conn, err1 := listen.Accept()
		if err1 != nil {
			return err1
		}
		tmpFd, tmpErr := utils.NewFdConn(conn, conf.IsEncrypt, conf.EncryptMethod, conf.EncryptToken)
		if tmpErr != nil {
			fmt.Printf("ReqServer处理加密连接失败：%v\n", tmpErr)
			continue
		}
		sConn := &leftConn{
			FdConn: tmpFd,
		}
		go reqServerProcess(sConn)
	}
}

func reqServerProcess(c *leftConn) {
	defer func() {
		if err := c.Close(); err != nil {
			c.Log(fmt.Sprintf("关闭：%v", err.Error()))
		}
	}()

	//todo 验证认证信息

	//握手信息 proxyTag targetAddr
	rMsg := utils.MsgDecode(c.Conn)
	if rMsg.Err != nil {
		c.Log(fmt.Sprintf("握手信息解析失败：%v", rMsg.Err.Error()), log.DebugError)
		return
	}
	tmpMsgData := &utils.StructMsg{}
	if err := json.Unmarshal(rMsg.Msg, tmpMsgData); err != nil {
		c.Log(fmt.Sprintf("握手信息解析失败：%v", err.Error()), log.DebugError)
		return
	}

	tunnel, tunnelErr := proxyMan.getRightConn(tmpMsgData.ProxyTag)
	if tunnelErr != nil {
		c.Log(fmt.Sprintf("握手失败，获取隧道失败：%v", tunnelErr.Error()), log.DebugError)
		return
	}

	if err := tunnel.Write(rMsg.Msg, rMsg.StreamId, rMsg.Action); err != nil {
		c.Log(fmt.Sprintf("握手失败，通过隧道转发数据失败：%v", err.Error()), log.DebugError)
		return
	}

	c.UKey = rMsg.StreamIdString()
	proxyMan.addLeftConn(c)

	for {
		select {
		case <-c.Ctx.Done():
			return
		default:
			rMsg = utils.MsgDecode(c.Conn)
			if rMsg.Err != nil {
				c.Log(fmt.Sprintf("断开：%v", rMsg.Err.Error()))
				if err := tunnel.Write([]byte("客户端断开"), rMsg.StreamId, []byte(utils.EventClose)); err != nil {
					c.Log("发送断开消息失败：%v", err.Error())
				}
				return
			}
			if err := tunnel.Write(rMsg.Msg, rMsg.StreamId, rMsg.Action); err != nil {
				c.Log("发送消息失败：%v", err.Error())
				return
			}
		}
	}
}
