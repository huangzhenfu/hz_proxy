package config

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"log"
	"sync"
)

type ConfCol struct {
	Debug  bool
	Req    reqConf
	MidReq midReqConf
	MidRes midResConf
	Res    resConf
}
type reqConf struct {
	HttpPort      int
	TcpPort       int
	MidPort       int
	MidHost       string
	EncryptMethod string
	EncryptToken  string
	IsEncrypt     bool
	ProxyTag      string
	Username      string
	Pwd           string
	TcpTargetAddr string
}
type midReqConf struct {
	Port          int
	EncryptMethod string
	EncryptToken  string
	IsEncrypt     bool
	Users         []userReq
}
type userReq struct {
	Username string
	Pwd      string
}

type proxyTag struct {
	Tag      string
	Dest     string
	PoolSize int
	Username string
	Pwd      string
}

type midResConf struct {
	Port          int
	EncryptMethod string
	EncryptToken  string
	IsEncrypt     bool
	ProxyTags     []proxyTag
}

type resConf struct {
	MidPort       int
	MidHost       string
	EncryptMethod string
	EncryptToken  string
	IsEncrypt     bool
	Username      string
	Pwd           string
	Tag           string
	PoolSize      int
}

var Conf ConfCol
var configLock = sync.RWMutex{}

func init() {
	// 初始化 Viper
	v := viper.New()
	v.SetConfigName("proxy")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

	// 读取配置
	if err := v.ReadInConfig(); err != nil {
		log.Fatalf("Error reading proxy file, %s", err)
	}

	// 初始解析配置到结构体
	if err := v.Unmarshal(&Conf); err != nil {
		log.Fatalf("无法解析配置到结构体: %v", err)
	}

	// 设置配置变化监视
	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("\n检测到配置文件变化:", e.Name)
		// 重新加载配置
		if err := v.ReadInConfig(); err != nil {
			log.Printf("重新加载配置失败: %v", err)
			return
		}
		// 加锁更新配置结构体
		configLock.Lock()
		defer configLock.Unlock()
		if err := v.Unmarshal(&Conf); err != nil {
			log.Printf("重新解析配置失败: %v", err)
			return
		}
		fmt.Println("配置已更新:")
	})
}
