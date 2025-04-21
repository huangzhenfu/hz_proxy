package mid

import (
	"encoding/json"
	"fmt"
	"hz_proxy/config"
	log2 "hz_proxy/log"
	"hz_proxy/utils"
	"log"
	"net"
)

type rightConn struct {
	*utils.FdConn
	Tag string `json:"tag" form:"tag" comment:"分组标签"`
}

func (s *rightConn) close() error {
	if err := s.FdConn.Close(); err != nil {
		return err
	}
	return nil
}

func ResServer() error {
	conf := config.Conf.MidRes
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", conf.Port))
	if err != nil {
		return err
	}
	defer func() {
		if err = listen.Close(); err != nil {
			log.Println(err)
		}
	}()
	for {
		conn, err1 := listen.Accept()
		if err1 != nil {
			return err1
		}
		tmpFd, tmpErr := utils.NewFdConn(conn, conf.IsEncrypt, conf.EncryptMethod, conf.EncryptToken)
		if tmpErr != nil {
			continue
		}
		sConn := &rightConn{
			FdConn: tmpFd,
		}
		go resServerProcess(sConn)
	}
}

func checkAuth(tag, username, pwd string) bool {
	retRes := false
	list := config.Conf.MidRes.ProxyTags
	for _, v := range list {
		if v.Tag == tag {
			if v.Username == username && v.Pwd == pwd {
				retRes = true
			}
			break
		}
	}
	return retRes
}

func checkAuthReq(username, pwd string) bool {
	retRes := false
	list := config.Conf.MidReq.Users
	for _, v := range list {
		if v.Username == username && v.Pwd == pwd {
			retRes = true
		}
		break
	}
	return retRes
}

func resServerProcess(c *rightConn) {
	defer func() {
		if err := c.close(); err != nil {
			log.Println(err.Error())
		}
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()

	//处理认证
	authMsg := utils.MsgDecode(c.Conn)
	if authMsg.Err != nil {
		return
	}
	authMsgData := &utils.StructMsg{}
	if err := json.Unmarshal(authMsg.Msg, authMsgData); err != nil {
		return
	}
	if !checkAuth(authMsgData.ProxyTag, authMsgData.Username, authMsgData.Password) {
		if err := c.Write([]byte("auth failed"), authMsg.StreamId, []byte(utils.EventAuth)); err != nil {
			log.Println(err.Error())
		}
		return
	}
	if err := c.Write([]byte("ok"), authMsg.StreamId, []byte(utils.EventAuth)); err != nil {
		log.Println(err.Error())
	}

	//认证成功，加入连接池
	c.Tag = authMsgData.ProxyTag
	proxyMan.addRightConn(c)

	//接收右侧响应代理发送过来的数据
	for {
		tMsg := utils.MsgDecode(c.Conn)
		if tMsg.Err != nil {
			log.Printf("响应代理端断开:%v", tMsg.Err.Error())
			return
		}
		lConn, lErr := proxyMan.getLeftConn(tMsg.StreamIdString())
		if lErr != nil {
			log2.Debug(fmt.Sprintf("句柄：%s，隧道转发目标服务数据失败,未获取到左侧连接：%v", tMsg.StreamIdString(), lErr.Error()))
			continue
		}
		lConn.Write(tMsg.Msg, tMsg.StreamId, tMsg.Action)
	}
}
