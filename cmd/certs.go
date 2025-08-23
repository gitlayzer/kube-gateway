package cmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

func ensureCerts(certPath, keyPath string) error {
	// 检查文件是否存在
	if _, err := os.Stat(certPath); err == nil {
		if _, err = os.Stat(keyPath); err == nil {
			log.Println("证书文件已存在，跳过生成步骤。")
			return nil
		}
	}

	log.Println("未找到 TLS 证书，正在自动生成新的证书...")

	// 创建证书存放目录
	certDir := filepath.Dir(certPath)
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("无法创建证书目录 %s: %w", certDir, err)
	}

	// 1. 生成 RSA 私钥
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("生成 RSA 私钥失败: %w", err)
	}

	// 2. 创建证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()), // 唯一的序列号
		Subject: pkix.Name{
			CommonName: "kube-gateway.local",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(1, 0, 0),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,

		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:    []string{"localhost"},
	}

	// 3. 创建自签名证书
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("创建证书失败: %w", err)
	}

	// 4. 将证书编码为 PEM 格式并保存到文件
	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("创建证书文件失败: %w", err)
	}
	defer certFile.Close()
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		return fmt.Errorf("编码证书到 PEM 格式失败: %w", err)
	}

	// 5. 将私钥编码为 PEM 格式并保存到文件
	keyFile, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("创建私钥文件失败: %w", err)
	}
	defer keyFile.Close()
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}); err != nil {
		return fmt.Errorf("编码私钥到 PEM 格式失败: %w", err)
	}

	log.Printf("✅ 成功生成并保存证书到 %s 和 %s", certPath, keyPath)
	return nil
}
