package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/good-yellow-bee/blazelog/pkg/config"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print the version, commit, and build time of blazelog.`,
	Run: func(cmd *cobra.Command, args []string) {
		if GetOutput() == "json" {
			info := config.GetBuildInfo()
			data, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Println(config.VersionString())
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
