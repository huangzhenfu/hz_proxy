package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "proxy [req|mid|res] [ARG...]",
	Short: "网络代理工具",
	Long:  `网络代理工具支持HTTP、HTTPS、TCP`,
}

func Execute() {
	rootCmd.AddCommand(
		reqCmd,
		midCmd,
		resCmd,
	)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}
