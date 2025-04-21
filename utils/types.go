package utils

import (
	"bufio"
	"context"
	"fmt"
	"github.com/google/uuid"
	"hz_proxy/log"
	"net"
	"strings"
	"sync"
	"time"
)

type FdConn struct {
	Conn       net.Conn
	CipherConn net.Conn
	Reader     *bufio.Reader
	STime      time.Time
	IsClose    bool
	Ctx        context.Context
	Cancel     context.CancelFunc
	Lock       sync.Mutex
	UKey       string
}

func NewFdConn(conn net.Conn, IsEncrypt bool, EncryptMethod, EncryptToken string) (*FdConn, error) {
	ctx, cancel := context.WithCancel(context.Background())
	oneConn := &FdConn{
		Conn:   conn,
		STime:  time.Now(),
		Ctx:    ctx,
		Cancel: cancel,
		Lock:   sync.Mutex{},
		UKey:   strings.Replace(uuid.New().String(), "-", "", -1),
	}
	if IsEncrypt {
		tmpConn, tmpErr := CipherConn(conn, EncryptMethod, EncryptToken)
		if tmpErr != nil {
			return nil, tmpErr
		}
		oneConn.CipherConn = tmpConn
		oneConn.Conn = tmpConn
		oneConn.Reader = bufio.NewReader(tmpConn)
	} else {
		oneConn.Reader = bufio.NewReader(conn)
	}
	return oneConn, nil
}

func (s *FdConn) Close() error {
	s.IsClose = true
	s.Cancel()
	return s.Conn.Close()
}

func (s *FdConn) Write(msg []byte, streamId []byte, action []byte) (retErr error) {
	defer func() {
		if err := recover(); err != nil {
			log.Debug(fmt.Sprintf("句柄:%s，发送消息错误：%v", s.UKey, err))
		}
		retErr = nil
	}()
	s.Lock.Lock()
	defer s.Lock.Unlock()
	sendMsg, sErr := MsgEncode(msg, streamId, action)
	if sErr != nil {
		return sErr
	}
	if s.CipherConn != nil {
		if _, err := s.CipherConn.Write(sendMsg); err != nil {
			return err
		}
		return nil
	}
	if _, err := s.Conn.Write(sendMsg); err != nil {
		return err
	}
	return nil
}

func (s *FdConn) Log(msg string, levels ...string) {
	log.Debug(fmt.Sprintf("句柄:%s，%s", s.UKey, msg), levels...)
}

type StructMsg struct {
	ProxyTag   string `json:"proxy_tag"`
	TargetAddr string `json:"target_addr"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}
