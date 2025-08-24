package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	// 引入 client-go 中专门处理 kubeconfig 的库
	"k8s.io/client-go/tools/clientcmd"
)

var removeCmd = &cobra.Command{
	Use:   "remove [cluster-name]",
	Short: "Remove a cluster configuration and automatically clean local kubeconfig",
	Args:  cobra.ExactArgs(1),
	Run:   runRemove,
}

func init() {
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) {
	clusterName := args[0]

	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("错误: 无法获取用户主目录: %v", err)
	}
	clusterDir := filepath.Join(home, ".kube-gateway", "clusters", clusterName)

	if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
		fmt.Printf("✅ 服务端配置 '%s' 不存在，无需清理。\n", clusterName)
	} else {
		if err := os.RemoveAll(clusterDir); err != nil {
			log.Fatalf("错误: 移除服务端配置目录失败: %v", err)
		}
		fmt.Printf("✅ 服务端配置 '%s' 已成功移除。\n", clusterName)
	}

	fmt.Println("\n🔄 正在自动清理本地 kubeconfig...")
	if err := cleanupKubeconfig(clusterName); err != nil {
		fmt.Printf("   ❌ 自动清理 kubeconfig 失败: %v\n", err)
		fmt.Println("   可能需要你手动编辑 ~/.kube/config 文件。")
	} else {
		fmt.Println("   ✅ 本地 kubeconfig 清理成功！")
	}

	fmt.Println("\n💡 如果服务正在运行，请执行 'kube-gateway reload' 来应用变更。")
}

func cleanupKubeconfig(clusterName string) error {
	kubeconfigPath := clientcmd.RecommendedHomeFile

	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		// 如果文件本身就不存在，那也无需清理
		if os.IsNotExist(err) {
			fmt.Println("   -> kubeconfig 文件不存在，无需清理。")
			return nil
		}
		return fmt.Errorf("加载 kubeconfig 文件失败: %w", err)
	}

	// 构造需要删除的 user 和 context 的名称
	userName := "user-for-" + clusterName
	contextName := "gateway-" + clusterName

	// 检查条目是否存在，如果不存在，则无需操作
	_, userExists := config.AuthInfos[userName]
	_, contextExists := config.Contexts[contextName]
	if !userExists && !contextExists {
		fmt.Printf("   -> 在 kubeconfig 中未找到与 '%s' 相关的配置，无需清理。\n", clusterName)
		return nil
	}

	// 在写入文件之前，确保父目录存在
	kubeconfigDir := filepath.Dir(kubeconfigPath)
	if err := os.MkdirAll(kubeconfigDir, 0755); err != nil {
		return fmt.Errorf("无法创建 .kube 目录 %s: %w", kubeconfigDir, err)
	}

	// 在写入前，先备份原始文件
	if _, err := os.Stat(kubeconfigPath); err == nil {
		if err := os.Rename(kubeconfigPath, kubeconfigPath+".bak"); err != nil {
			return fmt.Errorf("备份原始 kubeconfig 文件失败: %w", err)
		}
		fmt.Printf("   -> 已将原始配置备份到 %s.bak\n", kubeconfigPath)
	}

	// 从配置中删除 user 和 context
	delete(config.AuthInfos, userName)
	delete(config.Contexts, contextName)

	fmt.Printf("   -> 已删除 user '%s' 和 context '%s'。\n", userName, contextName)

	// 【重要】检查被删除的 context 是否是当前 context
	if config.CurrentContext == contextName {
		fmt.Printf("   -> 被删除的上下文是当前上下文，正在切换到其他上下文...\n")
		// 如果还有其他 context，就切换到第一个
		if len(config.Contexts) > 0 {
			for newCurrentContext := range config.Contexts {
				config.CurrentContext = newCurrentContext
				fmt.Printf("   -> 已将当前上下文切换到 '%s'。\n", newCurrentContext)
				break
			}
		} else {
			// 如果没有其他 context 了，就清空 current-context
			config.CurrentContext = ""
			fmt.Println("   -> 已无其他上下文，当前上下文已清空。")
		}
	}

	// 将修改后的配置写回文件
	if err := clientcmd.WriteToFile(*config, kubeconfigPath); err != nil {
		return fmt.Errorf("写入 kubeconfig 文件失败: %w", err)
	}

	return nil
}
