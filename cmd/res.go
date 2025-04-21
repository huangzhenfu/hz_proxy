package cmd

import (
	"github.com/spf13/cobra"
	"hz_proxy/res"
)

var resCmd = &cobra.Command{
	Use:   "res",
	Short: "目标响应服务代理",
	Long:  `目标响应服务代理`,
	Run: func(cmd *cobra.Command, args []string) {
		res.Run()
	},
}
