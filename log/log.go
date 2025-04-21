package log

import (
	"fmt"
	"hz_proxy/config"
	"log"
	"time"
)

const (
	DebugError = "error"
	DebugFatal = "fatal"
	DebugInfo  = "info"
)

func Debug(msg string, levels ...string) {
	if !config.Conf.Debug {
		return
	}

	level := DebugInfo
	if len(levels) > 0 {
		level = levels[0]
	}

	if level == DebugFatal {
		log.Fatal(msg)
		return
	}

	if level == DebugError {
		log.Println(msg)
		return
	}

	fmt.Printf("%s %s\n", time.Now().Format("2006/01/02 15:04:05"), msg)
}
