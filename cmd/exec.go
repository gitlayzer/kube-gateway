package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

var execCmd = &cobra.Command{
	Use:   "exec [cluster-name] -- [command...]",
	Short: "Execute commands on the specified cluster without switching kubectl context",
	Long: `This command allows the user to not change the current kubectl context,
Quickly execute commands on any cluster managed by kube-gateway.
Use the '--' separator to distinguish the parameters of kube-gateway from the command to be executed.

Example:
  kube-gateway exec dev -- kubectl get pods -n default
  kube-gateway exec staging -- helm list`,
	// 我们需要至少两个参数: 集群名称 和 -- 分隔符
	Args: cobra.MinimumNArgs(2),
	Run:  runExec,
}

func init() {
	rootCmd.AddCommand(execCmd)
}

func runExec(cmd *cobra.Command, args []string) {
	clusterName := args[0]

	// 找到 '--' 分隔符的位置
	dashPos := cmd.ArgsLenAtDash()
	if dashPos == -1 {
		log.Fatalf("错误: 请使用 '--' 分隔符来指定要执行的命令。")
	}

	// '--' 后面的所有内容都是要执行的命令及其参数
	commandAndArgs := args[dashPos:]
	if len(commandAndArgs) == 0 {
		log.Fatalf("错误: '--' 后面没有提供任何要执行的命令。")
	}
	command := commandAndArgs[0]
	commandArgs := commandAndArgs[1:]

	// 1. 获取 Token
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("错误: 无法获取用户主目录: %v", err)
	}
	tokenPath := filepath.Join(home, ".kube-gateway", "clusters", clusterName, "token")
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("错误: 找不到名为 '%s' 的集群配置。", clusterName)
		}
		log.Fatalf("错误: 无法读取集群 '%s' 的 token 文件: %v", clusterName, err)
	}
	token := strings.TrimSpace(string(tokenBytes))

	// 2. 创建一个临时的一次性 kubeconfig 文件
	tempKubeconfigFile, err := createTempKubeconfig(clusterName, token)
	if err != nil {
		log.Fatalf("错误: 创建临时 kubeconfig 文件失败: %v", err)
	}
	// 确保在函数退出时删除临时文件
	defer os.Remove(tempKubeconfigFile)

	// 3. 准备并执行子命令
	log.Printf("==> 正在集群 '%s' 上执行: %s %s\n", clusterName, command, strings.Join(commandArgs, " "))

	// 使用 os/exec 来准备子命令
	execCmd := exec.Command(command, commandArgs...)

	// 【核心】为子命令设置 KUBECONFIG 环境变量，并继承当前进程的其他环境变量
	execCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", tempKubeconfigFile))

	// 将子命令的输入、输出和错误流连接到当前终端
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	// 启动并等待命令执行完成
	if err := execCmd.Run(); err != nil {
		// exec.Run() 会在命令非零退出时返回错误，这通常是正常的（比如 kubectl get a-non-existent-pod）
		// 所以我们只打印错误，但不让 kube-gateway 本身以失败状态退出
		// log.Fatalf("执行命令失败: %v", err)
	}
}

// createTempKubeconfig 在系统临时目录中创建一个一次性的 kubeconfig 文件
func createTempKubeconfig(clusterName, token string) (string, error) {
	// 创建一个只包含我们所需上下文的全新配置对象
	config := api.NewConfig()

	gatewayClusterName := "my-gateway-exec"
	gatewayServerURL := "https://127.0.0.1:8443"

	// 读取 CA 证书以建立信任
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	caPath := filepath.Join(home, ".kube-gateway", "certs", "server.pem")
	caData, err := os.ReadFile(caPath)
	if err != nil {
		return "", fmt.Errorf("无法读取 CA 证书 %s: %w. 请先运行 'serve' 命令来生成证书。", caPath, err)
	}

	// 设置 Cluster
	cluster := api.NewCluster()
	cluster.Server = gatewayServerURL
	cluster.CertificateAuthorityData = caData
	config.Clusters[gatewayClusterName] = cluster

	// 设置 User
	userName := "user-for-" + clusterName
	user := api.NewAuthInfo()
	user.Token = token
	config.AuthInfos[userName] = user

	// 设置 Context
	contextName := "gateway-exec-" + clusterName
	context := api.NewContext()
	context.Cluster = gatewayClusterName
	context.AuthInfo = userName
	config.Contexts[contextName] = context

	// 设置 CurrentContext
	config.CurrentContext = contextName

	// 创建一个临时文件来保存这个配置
	tempFile, err := os.CreateTemp("", "kube-gateway-exec-*.yaml")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	// 将配置内容写入临时文件
	if err := clientcmd.WriteToFile(*config, tempFile.Name()); err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}
