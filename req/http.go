package req

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hz_proxy/config"
	"hz_proxy/log"
	"hz_proxy/utils"
	"net"
	"net/url"
	"strings"
)

func HttpClient() {
	conf := config.Conf.Req
	listen, err := net.Listen("tcp", fmt.Sprintf(":%v", conf.HttpPort))
	if err != nil {
		fmt.Printf("%v", err)
		return
	}
	defer listen.Close()
	for {
		conn, cErr := listen.Accept()
		if cErr != nil {
			fmt.Printf("%v", err)
			return
		}
		fdConn, _ := utils.NewFdConn(conn, false, "", "")
		go processHttp(fdConn)
	}
}

func processHttp(conn *utils.FdConn) {
	defer func() {
		conn.Close()
		if err := recover(); err != any(nil) {

		}
	}()

	conf := config.Conf.Req

	var b [4096]byte
	n, err := conn.Conn.Read(b[:]) //读取应用层的所有数据
	if err != nil || bytes.IndexByte(b[:], '\n') == -1 {
		conn.Log("读取应用层数据失败：" + err.Error())
		return
	}

	//解析http头信息
	var method, host, address, httpX string
	fmt.Sscanf(string(b[:bytes.IndexByte(b[:], '\n')]), "%s%s%s", &method, &host, &httpX)

	conn.Log(fmt.Sprintf("=>%s", host))

	hostPortURL, err := url.Parse(host)
	if err != nil {
		conn.Log("解析url失败：" + err.Error())
		return
	}
	if hostPortURL.Opaque == "443" { //https访问
		address = hostPortURL.Scheme + ":443"
	} else { //http访问
		if strings.Index(hostPortURL.Host, ":") == -1 { //host不带端口， 默认80
			address = hostPortURL.Host + ":80"
		} else {
			address = hostPortURL.Host
		}
	}

	//跟远程服务建立连接
	midTcpSerAddr := fmt.Sprintf("%s:%d", conf.MidHost, conf.MidPort)
	mConn, err := net.Dial("tcp", midTcpSerAddr)
	if err != nil {
		conn.Log(fmt.Sprintf("目标服务：%s,连接中间服务器失败: %v", address, err.Error()), log.DebugError)
		return
	}
	mFdConn, mFErr := utils.NewFdConn(mConn, conf.IsEncrypt, conf.EncryptMethod, conf.EncryptToken)
	if mFErr != nil {
		conn.Log(fmt.Sprintf("目标服务：%s,连接中间服务器成功，但连接加密失败: %v", address, mFErr.Error()), log.DebugError)
		return
	}
	defer mFdConn.Close()

	//认证：发送用户名和密码
	tmpMsg := utils.StructMsg{Username: conf.Username, Password: conf.Pwd}
	sendMsg, _ := json.Marshal(tmpMsg)
	if err = mFdConn.Write(sendMsg, []byte{}, []byte(utils.EventAuth)); err != nil {
		conn.Log(fmt.Sprintf("目标服务：%s,远程服务认证失败: %v", address, err.Error()), log.DebugError)
		return
	}
	authMsg := utils.MsgDecode(mFdConn.Conn)
	if authMsg.Err != nil {
		conn.Log(fmt.Sprintf("目标服务：%s,远程服务认证失败: %v", address, authMsg.Err.Error()), log.DebugError)
		return
	}
	if string(authMsg.Msg) != "ok" {
		conn.Log(fmt.Sprintf("目标服务：%s,远程服务认证失败: %v", address, string(authMsg.Msg)), log.DebugError)
		return
	}
	//连接建立成功
	conn.Log(fmt.Sprintf("目标服务：%s,远程服务认证成功", address))

	//请求和target建立连接
	proxyAndTarget, _ := json.Marshal(utils.StructMsg{ProxyTag: conf.ProxyTag, TargetAddr: address})
	if err = mFdConn.Write(proxyAndTarget, []byte(conn.UKey), []byte(utils.EventShake)); err != nil {
		conn.Log("目标服务：%s,向中间服务器发送target目标地址失败: %v", address, err.Error(), log.DebugError)
		return
	}
	conn.Log(fmt.Sprintf("目标服务：%s,发送握手消息", address))

	//等待握手信息
	retMsg := utils.MsgDecode(mFdConn.Conn)
	if retMsg.Err != nil {
		conn.Log(fmt.Sprintf("目标服务：%s,握手失败", retMsg.Err.Error()), log.DebugError)
		return
	}
	if string(retMsg.Msg) != "ok" {
		conn.Log(fmt.Sprintf("目标服务：%s,握手失败", string(retMsg.Msg)), log.DebugError)
		return
	}

	conn.Log(fmt.Sprintf("目标服务：%s,握手成功", address))

	if method == "CONNECT" {
		conn.Conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	} else {
		if err = mFdConn.Write(b[:n], []byte(conn.UKey), []byte(utils.EventTransfer)); err != nil {
			conn.Log(fmt.Sprintf("目标服务：%s,发送消息失败：%s", address, err.Error()))
			return
		}
		conn.Log(fmt.Sprintf("目标服务:%s,发送消息：%v字节", address, n))
	}

	//把数据转给中间服务器
	go func() {
		for {
			select {
			case <-conn.Ctx.Done():
				return
			default:
				buf := make([]byte, 4096)
				rLen, rErr := conn.Conn.Read(buf)
				if rErr != nil {
					if err = mFdConn.Write([]byte{}, []byte(conn.UKey), []byte(utils.EventClose)); err != nil {
					}
					conn.Log(fmt.Sprintf("目标服务：%s,主动断开：%s", address, rErr.Error()))
					return
				}
				if err = mFdConn.Write(buf[:rLen], []byte(conn.UKey), []byte(utils.EventTransfer)); err != nil {
					conn.Log(fmt.Sprintf("目标服务:%s,发送消息失败：%v", address, err.Error()))
					continue
				}
				conn.Log(fmt.Sprintf("目标服务:%s,发送消息：%v字节", address, rLen))
			}
		}
	}()

	//从中间服务器接收数据
	for {
		rMsg := utils.MsgDecode(mFdConn.Conn)
		if rMsg.Err != nil {
			conn.Log(fmt.Sprintf("目标服务:%s,中间服务断开:%v", address, rMsg.Err.Error()))
			return
		}
		switch rMsg.StreamIdString() {
		case utils.EventClose:
			conn.Log(fmt.Sprintf("目标服务:%s,收到断开消息", address))
			return
		}
		if _, err = conn.Conn.Write(rMsg.Msg); err != nil {
			return
		}
		conn.Log(fmt.Sprintf("目标服务:%s,收到消息:%v字节", address, rMsg.Len))
	}
}
