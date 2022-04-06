package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

// rootCmd 代表没有调用子命令时的基础命令
var rootCmd = &cobra.Command{
	Use:   "k8s-bark",
	Short: "k8s-bark is a tool for watching k8s cluster and push message to iphone",
	Long:  "A Service to watch Kubernetes Cluster and resourse events and status and push message to iphone",
}

// versionCmd 代表输入version时的基础命令
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of k8s-bark",
	Long:  `All software has versions. This is k8s-bark's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("v0.1")
	},
}

// Execute 将所有子命令添加到root命令并适当设置标志。
// 这由 main.main() 调用。它只需要对 rootCmd 调用一次。
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
