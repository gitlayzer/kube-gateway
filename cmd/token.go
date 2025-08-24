package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Management cluster authentication Token",
}

var rotateCmd = &cobra.Command{
	Use:   "rotate [cluster-name]",
	Short: "Rotate a new authentication for a specified cluster.",
	Args:  cobra.ExactArgs(1),
	Run:   runRotate,
}

func init() {
	// 将 rotateCmd 作为 tokenCmd 的子命令
	tokenCmd.AddCommand(rotateCmd)
	// 将父命令 tokenCmd 添加到根命令
	rootCmd.AddCommand(tokenCmd)
}

func runRotate(cmd *cobra.Command, args []string) {
	clusterName := args[0]

	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("错误: 无法获取用户主目录: %v", err)
	}
	clusterDir := filepath.Join(home, ".kube-gateway", "clusters", clusterName)
	tokenPath := filepath.Join(clusterDir, "token")

	// 1. 检查集群配置是否存在
	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		log.Fatalf("错误: 找不到名为 '%s' 的集群配置。", clusterName)
	}

	// 2. 生成新 Token 并覆盖旧文件
	newToken := uuid.New().String()
	if err := os.WriteFile(tokenPath, []byte(newToken), 0644); err != nil {
		log.Fatalf("错误: 写入新 token 文件失败: %v", err)
	}

	fmt.Printf("✅ 集群 '%s' 的 Token 已成功轮换。\n", clusterName)
	fmt.Printf("   新 Token: %s\n", newToken)

	// 3. 自动更新本地 kubeconfig
	fmt.Println("\n🔄 正在自动更新本地 kubeconfig...")
	if err := updateKubeconfigForRotation(clusterName, newToken); err != nil {
		fmt.Printf("   ❌ 自动更新 kubeconfig 失败: %v\n", err)
		fmt.Println("   请手动编辑 ~/.kube/config 文件。")
	} else {
		fmt.Println("   ✅ 本地 kubeconfig 更新成功！")
	}

	fmt.Println("\n💡 如果服务正在运行，请执行 'kube-gateway reload' 来应用变更。")
}

func updateKubeconfigForRotation(clusterName, newToken string) error {
	kubeconfigPath := clientcmd.RecommendedHomeFile

	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("kubeconfig 文件不存在，无法更新")
		}
		return fmt.Errorf("加载 kubeconfig 文件失败: %w", err)
	}

	userName := "user-for-" + clusterName

	// 检查对应的 user 是否存在
	userInfo, exists := config.AuthInfos[userName]
	if !exists {
		return fmt.Errorf("在 kubeconfig 中找不到名为 '%s' 的用户配置", userName)
	}

	// 更新 token
	userInfo.Token = newToken

	// 备份原始文件
	if _, err := os.Stat(kubeconfigPath); err == nil {
		if err := os.Rename(kubeconfigPath, kubeconfigPath+".bak"); err != nil {
			return fmt.Errorf("备份原始 kubeconfig 文件失败: %w", err)
		}
		fmt.Printf("   -> 已将原始配置备份到 %s.bak\n", kubeconfigPath)
	}

	// 将修改后的配置写回文件
	if err := clientcmd.WriteToFile(*config, kubeconfigPath); err != nil {
		return fmt.Errorf("写入 kubeconfig 文件失败: %w", err)
	}

	return nil
}
