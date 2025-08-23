package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	// 引入 client-go 中专门处理 kubeconfig 的库
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

var addCmd = &cobra.Command{
	Use:   "add [cluster-name] [source-kubeconfig-path]",
	Short: "添加一个新的集群配置，并自动更新本地 kubeconfig",
	Args:  cobra.ExactArgs(2),
	Run:   runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) {
	clusterName := args[0]
	sourceKubeconfigPath := args[1]

	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("错误: 无法获取用户主目录: %v", err)
	}
	clusterDir := filepath.Join(home, ".kube-gateway", "clusters", clusterName)

	// =========================================================
	//  1. 服务端配置
	// =========================================================
	if _, err := os.Stat(clusterDir); !os.IsNotExist(err) {
		log.Fatalf("错误: 名为 '%s' 的集群已存在于 %s", clusterName, clusterDir)
	}
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		log.Fatalf("错误: 创建集群目录失败: %v", err)
	}
	if err := copyFile(sourceKubeconfigPath, filepath.Join(clusterDir, "config")); err != nil {
		log.Fatalf("错误: 复制 kubeconfig 文件失败: %v", err)
	}
	newToken := uuid.New().String()
	if err := os.WriteFile(filepath.Join(clusterDir, "token"), []byte(newToken), 0644); err != nil {
		log.Fatalf("错误: 写入 token 文件失败: %v", err)
	}

	fmt.Println("✅ 服务端配置已成功添加！")
	fmt.Printf("   集群名称: %s\n", clusterName)
	fmt.Printf("   配置位置: %s\n", clusterDir)
	fmt.Printf("   生成的 Token: %s\n", newToken)

	// =========================================================
	//  2. 客户端 kubeconfig 自动更新
	// =========================================================
	fmt.Println("\n🔄 正在自动更新本地 kubeconfig...")
	if err := updateKubeconfig(clusterName, newToken); err != nil {
		fmt.Printf("   ❌ 自动更新 kubeconfig 失败: %v\n", err)
		fmt.Println("   请手动配置你的 ~/.kube/config 文件。")
	} else {
		fmt.Println("   ✅ 本地 kubeconfig 更新成功！")
		fmt.Printf("   已添加新的上下文 '%s' 并设为当前上下文。\n", "gateway-"+clusterName)
	}

	fmt.Println("\n💡 如果服务正在运行，请执行 'kube-gateway reload' 来应用变更。")
}

// updateKubeconfig 是一个辅助函数，负责所有 kubeconfig 的读写操作
func updateKubeconfig(clusterName, token string) error {
	// clientcmd.RecommendedHomeFile 是获取 ~/.kube/config 路径的标准方法
	kubeconfigPath := clientcmd.RecommendedHomeFile

	// 加载现有的 config 文件，如果不存在，会创建一个空的
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("加载 kubeconfig 文件失败: %w", err)
	}
	if config == nil {
		config = api.NewConfig()
	}

	// 定义我们网关的 cluster 信息 (可以复用)
	gatewayClusterName := "kube-gateway"
	gatewayServerURL := "https://127.0.0.1:8443"

	// 检查网关 cluster 是否已存在，不存在则添加
	gatewayCluster, exists := config.Clusters[gatewayClusterName]
	if !exists {
		gatewayCluster = api.NewCluster()
	}

	gatewayCluster.Server = gatewayServerURL

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("无法获取用户主目录: %w", err)
	}
	caPath := filepath.Join(home, ".kube-gateway", "certs", "server.pem")
	caData, err := os.ReadFile(caPath)
	if err != nil {
		return fmt.Errorf("无法读取 CA 证书 %s: %w. 请先运行 'serve' 命令来生成证书。", caPath, err)
	}
	gatewayCluster.CertificateAuthorityData = caData
	gatewayCluster.InsecureSkipTLSVerify = false

	config.Clusters[gatewayClusterName] = gatewayCluster

	// 创建新的 user 条目
	userName := "user-for-" + clusterName
	user := api.NewAuthInfo()
	user.Token = token
	config.AuthInfos[userName] = user

	// 创建新的 context 条目
	contextName := "gateway-" + clusterName
	context := api.NewContext()
	context.Cluster = gatewayClusterName
	context.AuthInfo = userName
	config.Contexts[contextName] = context

	// 将当前上下文切换到新创建的
	config.CurrentContext = contextName

	// 在写入文件之前，确保父目录存在
	kubeconfigDir := filepath.Dir(kubeconfigPath)
	if err := os.MkdirAll(kubeconfigDir, 0755); err != nil {
		return fmt.Errorf("无法创建 .kube 目录 %s: %w", kubeconfigDir, err)
	}

	// 在写入前，先备份原始文件，这是一个非常好的习惯
	if _, err := os.Stat(kubeconfigPath); err == nil {
		if err := os.Rename(kubeconfigPath, kubeconfigPath+".bak"); err != nil {
			return fmt.Errorf("备份原始 kubeconfig 文件失败: %w", err)
		}
		fmt.Printf("   -> 已将原始配置备份到 %s.bak\n", kubeconfigPath)
	}

	// 将修改后的配置写回文件
	// clientcmd.WriteToFile 会处理好所有文件权限和格式问题
	if err := clientcmd.WriteToFile(*config, kubeconfigPath); err != nil {
		return fmt.Errorf("写入 kubeconfig 文件失败: %w", err)
	}

	return nil
}

// copyFile 辅助函数
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
