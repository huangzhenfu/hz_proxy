package utils

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/shadowsocks/go-shadowsocks2/core"
	"io"
	"net"
	"sync"
	"time"
)

type corkedConn struct {
	net.Conn
	bufw   *bufio.Writer
	corked bool
	delay  time.Duration
	err    error
	lock   sync.Mutex
	once   sync.Once
}

func timedCork(c net.Conn, d time.Duration, bufSize int) net.Conn {
	return &corkedConn{
		Conn:   c,
		bufw:   bufio.NewWriterSize(c, bufSize),
		corked: true,
		delay:  d,
	}
}

func CipherConn(conn net.Conn, encryptMethod, encryptToken string) (net.Conn, error) {
	ciph, err := core.PickCipher(encryptMethod, []byte{}, encryptToken)
	if err != nil {
		return nil, err
	}
	conn = timedCork(conn, 10*time.Millisecond, 4096)
	return ciph.StreamConn(conn), nil
}

type MsgCol struct {
	Len      int32  `comment:"消息长度（4字节)"`
	StreamId []byte `comment:"流标志(32字节) "`
	Msg      []byte `comment:"消息体"`
	Err      error  `comment:"错误信息"`
	Action   []byte `comment:"(2字节)消息动作"`
}

func (m MsgCol) StreamIdString() string {
	return string(filterMsgByte(m.StreamId))
}

func (m MsgCol) ActionString() string {
	return string(filterMsgByte(m.Action))
}

// MsgEncode 消息长度（4字节）+ streamId（32字节） + action(2字节) + 消息
func MsgEncode(message []byte, streamId []byte, action []byte) ([]byte, error) {
	var pkg = new(bytes.Buffer)

	// 消息长度（4字节）
	if err := binary.Write(pkg, binary.LittleEndian, int32(len(message))); err != nil {
		return nil, err
	}

	//tag1(32字节)
	tmpStreamId := make([]byte, 32)
	copy(tmpStreamId, streamId) // 剩余部分自动填充 0
	if err := binary.Write(pkg, binary.LittleEndian, tmpStreamId); err != nil {
		return nil, err
	}

	//action(2字节)
	tmpAction := make([]byte, 2)
	copy(tmpAction, action) // 剩余部分自动填充 0
	if err := binary.Write(pkg, binary.LittleEndian, tmpAction); err != nil {
		return nil, err
	}

	// 写入消息实体
	if err := binary.Write(pkg, binary.LittleEndian, message); err != nil {
		return nil, err
	}
	return pkg.Bytes(), nil
}

// MsgDecode 消息长度（4字节）+ streamId（32字节） + action(2字节) + 消息
func MsgDecode(conn net.Conn) (ret MsgCol) {
	logRetMsg := func(tmp MsgCol) {
		//fmt.Printf(`
		//	解析的消息如下：
		//	消息长度：%v
		//	streamId：%v
		//	Action：%v
		//`, tmp.Len, tmp.StreamIdString(), tmp.ActionString())
	}

	header := make([]byte, 38)
	if err := readMsgWithTimeout(conn, header, 60*time.Second); err != nil {
		ret.Err = err
		logRetMsg(ret)
		return
	}
	ret.Len = int32(binary.LittleEndian.Uint32(header[0:4]))
	ret.StreamId = header[4:36]
	ret.Action = header[36:38]

	ret.Msg = make([]byte, ret.Len)
	if err := readMsgWithTimeout(conn, ret.Msg, 60*time.Second); err != nil {
		ret.Err = err
		logRetMsg(ret)
		return
	}
	return
}

func readMsgWithTimeout(conn net.Conn, buf []byte, timeout time.Duration) error {
	// 设置读取超时（绝对时间）
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	defer conn.SetReadDeadline(time.Time{}) // 清除超时
	// 尝试完整读取数据
	if _, err := io.ReadFull(conn, buf); err != nil {
		// 检查是否是超时错误
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return fmt.Errorf("读取超时: %v", timeout)
		}
		return err
	}
	return nil
}

func filterMsgByte(msg []byte) []byte {
	ret := make([]byte, 0)
	for _, b := range msg {
		if b == 0 {
			break
		}
		ret = append(ret, b)
	}
	return ret
}
