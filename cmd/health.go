package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type HealthStatus struct {
	ClusterName string
	Status      string
	Version     string
	Latency     string
	Error       error
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check health and delays for all configured clusters",
	Run:   runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

func runHealth(cmd *cobra.Command, args []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("错误: 无法获取用户主目录: %v", err)
	}
	clustersDir := filepath.Join(home, ".kube-gateway", "clusters")

	if _, err := os.Stat(clustersDir); os.IsNotExist(err) {
		fmt.Println("没有找到任何集群配置。请使用 'kube-gateway add' 命令添加一个。")
		return
	}

	fmt.Println("正在并发检查所有集群的健康状况...")

	var clusterDirs []string
	// 首先，收集所有集群目录的路径
	err = filepath.WalkDir(clustersDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != clustersDir {
			clusterDirs = append(clusterDirs, path)
			return filepath.SkipDir
		}
		return nil
	})

	if err != nil {
		log.Fatalf("错误: 遍历集群目录时出错: %v", err)
	}

	if len(clusterDirs) == 0 {
		fmt.Println("没有找到任何集群配置。")
		return
	}

	// 使用 channel 来收集并发执行的结果
	resultsChan := make(chan HealthStatus, len(clusterDirs))
	var wg sync.WaitGroup

	for _, dir := range clusterDirs {
		wg.Add(1)
		// 为每个集群启动一个 goroutine 进行健康检查
		go func(clusterPath string) {
			defer wg.Done()
			resultsChan <- checkClusterHealth(clusterPath)
		}(dir)
	}

	// 等待所有 goroutine 完成
	wg.Wait()
	close(resultsChan)

	// 从 channel 中收集所有结果
	var statuses []HealthStatus
	for status := range resultsChan {
		statuses = append(statuses, status)
	}

	// 打印结果表格
	printHealthTable(statuses)
}

// checkClusterHealth 负责检查单个集群的健康状况
func checkClusterHealth(clusterPath string) HealthStatus {
	clusterName := filepath.Base(clusterPath)
	configPath := filepath.Join(clusterPath, "config")

	status := HealthStatus{ClusterName: clusterName}

	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		status.Status = "❌ DOWN"
		status.Error = fmt.Errorf("无法加载配置: %w", err)
		return status
	}

	// 创建一个 clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		status.Status = "❌ DOWN"
		status.Error = fmt.Errorf("无法创建客户端: %w", err)
		return status
	}

	// 测量 API 请求延迟
	startTime := time.Now()
	// 设置一个较短的超时，避免长时间等待
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverVersion, err := clientset.Discovery().ServerVersion()
	latency := time.Since(startTime)

	if err != nil {
		status.Status = "❌ DOWN"
		status.Error = err
		status.Latency = "-"
		return status
	}

	status.Status = "✅ UP"
	status.Version = serverVersion.GitVersion
	status.Latency = latency.Round(time.Millisecond).String() // 格式化延迟为毫秒

	return status
}

// printHealthTable 使用 fmt.Printf 打印格式化的表格
func printHealthTable(statuses []HealthStatus) {
	// 定义表头和格式
	headerFormat := "%-25s %-10s %-20s %-10s %s\n"
	rowFormat := "%-25s %-10s %-20s %-10s %v\n"

	fmt.Println("\n集群健康检查结果:")
	fmt.Printf(headerFormat, "集群名称", "状态", "K8S 版本", "延迟", "错误信息")
	fmt.Printf(headerFormat, strings.Repeat("-", 25), strings.Repeat("-", 10), strings.Repeat("-", 20), strings.Repeat("-", 10), strings.Repeat("-", 30))

	for _, s := range statuses {
		if s.Error != nil {
			fmt.Printf(rowFormat, s.ClusterName, s.Status, "-", s.Latency, s.Error)
		} else {
			fmt.Printf(rowFormat, s.ClusterName, s.Status, s.Version, s.Latency, "")
		}
	}
}
