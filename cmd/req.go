package cmd

import (
	"github.com/spf13/cobra"
	"hz_proxy/req"
)

var reqCmd = &cobra.Command{
	Use:   "proxy req",
	Short: "客户端代理服务",
	Long:  `req`,
	Run: func(cmd *cobra.Command, args []string) {
		go req.TcpClient()
		req.HttpClient()
	},
}
