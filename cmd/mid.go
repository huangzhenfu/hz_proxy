package cmd

import (
	"github.com/spf13/cobra"
	"hz_proxy/mid"
)

var midCmd = &cobra.Command{
	Use:   "mid",
	Short: "mid",
	Long:  `mid`,
	Run: func(cmd *cobra.Command, args []string) {
		mid.Run()
	},
}
