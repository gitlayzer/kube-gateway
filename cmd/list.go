package cmd

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

type ClusterDisplayInfo struct {
	Name        string
	TokenSuffix string
	APIServer   string
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List the cluster list that has been added",
	Run:   runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("错误: 无法获取用户主目录: %v", err)
	}
	clustersDir := filepath.Join(home, ".kube-gateway", "clusters")

	if _, err := os.Stat(clustersDir); os.IsNotExist(err) {
		fmt.Println("没有找到任何集群配置。请使用 'kube-gateway add' 命令添加一个。")
		return
	}

	var clustersInfo []ClusterDisplayInfo

	// [ 这部分数据收集逻辑与之前完全相同，无需改动 ]
	err = filepath.WalkDir(clustersDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != clustersDir {
			clusterName := d.Name()
			info := ClusterDisplayInfo{Name: clusterName}
			tokenPath := filepath.Join(path, "token")
			tokenBytes, err := os.ReadFile(tokenPath)
			if err != nil {
				info.TokenSuffix = "Error"
			} else {
				token := strings.TrimSpace(string(tokenBytes))
				if len(token) > 8 {
					info.TokenSuffix = "..." + token[len(token)-8:]
				} else {
					info.TokenSuffix = token
				}
			}
			configPath := filepath.Join(path, "config")
			config, err := clientcmd.LoadFromFile(configPath)
			if err != nil {
				info.APIServer = "Error reading config file"
			} else {
				if len(config.Clusters) > 0 {
					for _, cluster := range config.Clusters {
						info.APIServer = cluster.Server
						break
					}
				} else {
					info.APIServer = "Not Found"
				}
			}
			clustersInfo = append(clustersInfo, info)
			return filepath.SkipDir
		}
		return nil
	})

	if err != nil {
		log.Fatalf("错误: 遍历集群目录时出错: %v", err)
	}

	if len(clustersInfo) == 0 {
		fmt.Println("没有找到任何集群配置。请使用 'kube-gateway add' 命令添加一个。")
		return
	}

	// 1. 定义表头
	headerFormat := "%-25s %-25s %s\n"
	fmt.Printf(headerFormat, "集群名称 (Name)", "Token 后缀 (Token Suffix)", "后端 API 服务器 (Backend API Server)")

	// 2. 打印分隔线
	fmt.Printf(headerFormat, strings.Repeat("-", 25), strings.Repeat("-", 25), strings.Repeat("-", 40))

	// 3. 遍历数据并按相同格式打印
	for _, info := range clustersInfo {
		fmt.Printf(headerFormat, info.Name, info.TokenSuffix, info.APIServer)
	}
}
