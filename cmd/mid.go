package cmd

import (
	"github.com/spf13/cobra"
	"hz_proxy/mid"
)

var midCmd = &cobra.Command{
	Use:   "mid",
	Short: "中间代理服务",
	Long:  `中间代理服务`,
	Run: func(cmd *cobra.Command, args []string) {
		mid.Run()
	},
}
