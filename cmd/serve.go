package cmd

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// authHeaderStrippingTransport 是一个实现了 http.RoundTripper 接口的结构体。
// 它的作用是在将请求传递给其底层的 transport 之前，先删除请求中的 "Authorization" Header。
type authHeaderStrippingTransport struct {
	// underlyingTransport 字段保存了真正用来发送请求的原始 transport
	underlyingTransport http.RoundTripper
}

// RoundTrip 实现了 http.RoundTripper 接口的核心方法。
// 每个通过这个 transport 的请求都会先经过这个方法。
func (t *authHeaderStrippingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 【关键逻辑】在请求被真正发送出去之前，删除原始的 Authorization Header
	req.Header.Del("Authorization")
	// 然后，将处理过的请求交给我们包装的底层 transport 去执行
	return t.underlyingTransport.RoundTrip(req)
}

var (
	proxyMap   map[string]*httputil.ReverseProxy
	proxyMutex sync.RWMutex
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the API gateway server (the TLS certificate will be automatically generated if needed)",
	Run:   runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("错误: 无法获取用户主目录: %v", err)
	}

	// 统一管理所有程序生成的文件路径
	certsDir := filepath.Join(home, ".kube-gateway", "certs")
	pidDir := filepath.Join(home, ".kube-gateway", "pid")
	certPath := filepath.Join(certsDir, "server.pem")
	keyPath := filepath.Join(certsDir, "server.key")
	pidFile := filepath.Join(pidDir, "kube-gateway.pid")

	// 确保证书存在
	if err := ensureCerts(certPath, keyPath); err != nil {
		log.Fatalf("处理 TLS 证书时出错: %v", err)
	}

	// 确保 PID 目录存在并写入 PID 文件
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		log.Fatalf("错误: 无法创建 PID 目录 %s: %v", pidDir, err)
	}
	pid := os.Getpid()
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		log.Fatalf("无法写入 PID 文件: %v", err)
	}
	defer os.Remove(pidFile)

	// 初始化加载代理配置
	if err := loadConfigAndProxies(); err != nil {
		log.Fatalf("初始化加载配置失败: %v", err)
	}

	// 启动信号监听器以支持热加载
	go handleSignals()

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Any("/*proxyPath", handleRequestWithGin)

	listenAddr := "0.0.0.0:8443"
	log.Printf("正在启动 kube-gateway HTTPS 服务器于 %s (PID: %d)", listenAddr, pid)
	if err := router.RunTLS(listenAddr, certPath, keyPath); err != nil && err != http.ErrServerClosed {
		log.Fatalf("启动 HTTPS 服务失败: %v", err)
	}
}

func loadConfigAndProxies() error {
	log.Println("正在扫描集群配置并重建代理...")
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("无法获取用户主目录: %w", err)
	}
	clustersDir := filepath.Join(home, ".kube-gateway", "clusters")

	if _, err := os.Stat(clustersDir); os.IsNotExist(err) {
		log.Printf("集群目录 %s 不存在。没有加载任何集群。", clustersDir)
		proxyMutex.Lock()
		proxyMap = make(map[string]*httputil.ReverseProxy)
		proxyMutex.Unlock()
		return nil
	}

	newProxyMap := make(map[string]*httputil.ReverseProxy)

	err = filepath.WalkDir(clustersDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != clustersDir {
			clusterName := d.Name()
			tokenPath := filepath.Join(path, "token")
			configPath := filepath.Join(path, "config")

			tokenBytes, err := os.ReadFile(tokenPath)
			if err != nil {
				log.Printf("警告: 无法读取集群 %s 的 token 文件: %v. 已跳过.", clusterName, err)
				return nil
			}
			token := strings.TrimSpace(string(tokenBytes))

			restConfig, err := clientcmd.BuildConfigFromFlags("", configPath)
			if err != nil {
				log.Printf("警告: 无法为集群 %s 构建配置: %v. 已跳过.", clusterName, err)
				return nil
			}

			backendTransport, err := rest.TransportFor(restConfig)
			if err != nil {
				log.Printf("警告: 无法为集群 %s 创建 transport: %v. 已跳过.", clusterName, err)
				return nil
			}

			targetUrl, err := url.Parse(restConfig.Host)
			if err != nil {
				log.Printf("警告: 无法解析集群 %s 的目标 URL: %v. 已跳过.", clusterName, err)
				return nil
			}

			proxy := httputil.NewSingleHostReverseProxy(targetUrl)
			proxy.Transport = &authHeaderStrippingTransport{underlyingTransport: backendTransport}

			newProxyMap[token] = proxy
			return filepath.SkipDir
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("遍历集群目录时出错: %w", err)
	}

	proxyMutex.Lock()
	proxyMap = newProxyMap
	proxyMutex.Unlock()

	log.Printf("配置加载完毕。当前有 %d 个集群代理处于活动状态。", len(newProxyMap))
	return nil
}

func handleSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)
	for {
		<-c
		log.Println("收到 SIGHUP 信号，尝试重新加载配置...")
		if err := loadConfigAndProxies(); err != nil {
			log.Printf("错误: 重载配置失败: %v", err)
		}
	}
}

func handleRequestWithGin(c *gin.Context) {
	authHeader := c.Request.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		c.String(http.StatusUnauthorized, "未授权: 缺少 Bearer Token")
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	proxyMutex.RLock()
	proxy, found := proxyMap[token]
	proxyMutex.RUnlock()
	if !found {
		c.String(http.StatusUnauthorized, "未授权: 无效的 Token")
		return
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}
