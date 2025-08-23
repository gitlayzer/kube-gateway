package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
)

var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload the configuration of the running gateway server",
	Run:   runReload,
}

func init() {
	rootCmd.AddCommand(reloadCmd)
}

func runReload(cmd *cobra.Command, args []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("错误: 无法获取用户主目录: %v", err)
	}

	// 从统一的位置读取 PID 文件
	pidFile := filepath.Join(home, ".kube-gateway", "pid", "kube-gateway.pid")

	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		log.Fatalf("错误: 无法读取 PID 文件 '%s'。服务是否正在运行? 错误: %v", pidFile, err)
	}

	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		log.Fatalf("在 '%s' 文件中发现无效的 PID: %v", pidFile, err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		log.Fatalf("无法找到 PID 为 %d 的进程: %v", pid, err)
	}

	if err := process.Signal(syscall.SIGHUP); err != nil {
		log.Fatalf("向 PID 为 %d 的进程发送 SIGHUP 信号失败: %v. 进程是否仍在运行?", pid, err)
	}

	fmt.Printf("✅ SIGHUP 信号已成功发送至 kube-gateway 进程 (PID %d)。\n", pid)
	fmt.Println("   请检查服务器日志以确认配置已重新加载。")
}
