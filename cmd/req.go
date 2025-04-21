package cmd

import (
	"github.com/spf13/cobra"
	"hz_proxy/req"
)

var reqCmd = &cobra.Command{
	Use:   "req",
	Short: "req",
	Long:  `req`,
	Run: func(cmd *cobra.Command, args []string) {
		go req.TcpClient()
		req.HttpClient()
	},
}
