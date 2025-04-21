package cmd

import (
	"github.com/spf13/cobra"
	"hz_proxy/res"
)

var resCmd = &cobra.Command{
	Use:   "res",
	Short: "res",
	Long:  `res`,
	Run: func(cmd *cobra.Command, args []string) {
		res.Run()
	},
}
