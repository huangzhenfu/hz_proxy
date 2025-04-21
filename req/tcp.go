package req

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hz_proxy/config"
	log2 "hz_proxy/log"
	"hz_proxy/utils"
	"net"
)

func TcpClient() {
	conf := config.Conf.Req
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", conf.TcpPort))
	if err != nil {
		fmt.Printf("%v", err)
		return
	}
	defer func() {
		if err = listen.Close(); err != nil {
			fmt.Println(err)
		}
	}()
	for {
		conn, err1 := listen.Accept()
		if err1 != nil {
			fmt.Printf("%v", err1)
			return
		}
		fdConn, _ := utils.NewFdConn(conn, false, "", "")
		go processTcp(fdConn)
	}
}
func processTcp(conn *utils.FdConn) {
	defer func() {
		defer conn.Close()
		if err := recover(); err != any(nil) {

		}
	}()

	conf := config.Conf.Req

	address, addrErr := tcpTargetAddr(conn)
	if addrErr != nil {
		conn.Log(fmt.Sprintf("解析协议失败，未获取到远程地址: %v", addrErr.Error()), log2.DebugError)
		return
	}

	//跟远程服务建立连接
	midTcpSerAddr := fmt.Sprintf("%s:%d", conf.MidHost, conf.MidPort)
	mConn, err := net.Dial("tcp", midTcpSerAddr)
	if err != nil {
		conn.Log(fmt.Sprintf("目标服务：%s,连接中间服务器失败: %v", address, err.Error()), log2.DebugError)
		return
	}
	mFdConn, mFErr := utils.NewFdConn(mConn, conf.IsEncrypt, conf.EncryptMethod, conf.EncryptToken)
	if mFErr != nil {
		conn.Log(fmt.Sprintf("目标服务：%s,连接中间服务器成功，但连接加密失败: %v", address, mFErr.Error()), log2.DebugError)
		return
	}
	defer mFdConn.Close()

	//todo 远程服务认证

	//请求和target建立连接
	proxyAndTarget, _ := json.Marshal(utils.StructMsg{ProxyTag: conf.ProxyTag, TargetAddr: address})
	if err = mFdConn.Write(proxyAndTarget, []byte(conn.UKey), []byte(utils.EventShake)); err != nil {
		conn.Log("目标服务：%s,向中间服务器发送target目标地址失败: %v", address, err.Error(), log2.DebugError)
		return
	}
	conn.Log(fmt.Sprintf("目标服务：%s,发送握手消息", address))

	//等待握手信息
	retMsg := utils.MsgDecode(mFdConn.Conn)
	if retMsg.Err != nil {
		conn.Log(fmt.Sprintf("目标服务：%s,握手失败", retMsg.Err.Error()), log2.DebugError)
		return
	}
	if string(retMsg.Msg) != "ok" {
		conn.Log(fmt.Sprintf("目标服务：%s,握手失败", string(retMsg.Msg)), log2.DebugError)
		return
	}

	conn.Log(fmt.Sprintf("目标服务：%s,握手成功", address))

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

func tcpTargetAddr(conn *utils.FdConn) (address string, pErr error) {
	address = "106.15.53.9:3306"

	//buf := make([]byte, 256)
	//n, err := io.ReadAtLeast(conn.Reader, buf, 4)
	//if err != nil {
	//	log.Printf("Read error: %v", err)
	//	return
	//}
	//
	//if isMySQL(buf[:n]) {
	//
	//}
	return address, nil
}

func isHTTP(data []byte) bool {
	methods := [][]byte{
		[]byte("GET "), []byte("POST "), []byte("PUT "),
		[]byte("HEAD "), []byte("DELETE "), []byte("OPTIONS "),
		[]byte("CONNECT "), []byte("HTTP/"),
	}
	for _, method := range methods {
		if bytes.HasPrefix(data, method) {
			return true
		}
	}
	return false
}

func isMySQL(data []byte) bool {
	if len(data) > 4 && data[4] == 0x0a {
		return true
	}
	return false
}
