package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"lxdapi/pkg/logger"

	"go.uber.org/zap"
)

const (
	SSL_CERTS_DIR = "nginx/certs"
)

type SSLCertManager struct {
	certsDir string
}

var CertManager *SSLCertManager

func InitSSLCertManager() error {
	baseDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取工作目录失败: %v", err)
	}

	certsDir := filepath.Join(baseDir, SSL_CERTS_DIR)
	if err := os.MkdirAll(certsDir, 0755); err != nil {
		return fmt.Errorf("创建证书目录失败: %v", err)
	}

	CertManager = &SSLCertManager{
		certsDir: certsDir,
	}

	ctx := context.Background()
	lc := &logger.Context{Action: "init_ssl_cert_manager"}
	ctx = logger.NewContext(ctx, lc)
	logger.Global.Info(ctx, "SSL证书管理器初始化成功", zap.String("dir", certsDir))

	return nil
}

func (m *SSLCertManager) GetCertPath(domain string) string {
	return filepath.Join(m.certsDir, domain+".crt")
}

func (m *SSLCertManager) GetKeyPath(domain string) string {
	return filepath.Join(m.certsDir, domain+".key")
}

func (m *SSLCertManager) SaveCustomCert(domain, certContent, keyContent string) (string, string, error) {
	ctx := context.Background()
	lc := &logger.Context{Action: "save_custom_cert"}
	ctx = logger.NewContext(ctx, lc)

	if certContent == "" || keyContent == "" {
		return "", "", fmt.Errorf("证书或私钥内容为空")
	}

	certPath := m.GetCertPath(domain)
	keyPath := m.GetKeyPath(domain)

	if err := os.WriteFile(certPath, []byte(certContent), 0644); err != nil {
		return "", "", fmt.Errorf("写入证书文件失败: %v", err)
	}

	if err := os.WriteFile(keyPath, []byte(keyContent), 0600); err != nil {
		os.Remove(certPath)
		return "", "", fmt.Errorf("写入私钥文件失败: %v", err)
	}

	logger.Global.Info(ctx, "自定义证书保存成功",
		zap.String("domain", domain),
		zap.String("cert", certPath),
		zap.String("key", keyPath))

	return certPath, keyPath, nil
}

func (m *SSLCertManager) GenerateSelfSignedCert(domain string) (string, string, error) {
	ctx := context.Background()
	lc := &logger.Context{Action: "generate_self_signed_cert"}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Info(ctx, "开始生成自签名证书", zap.String("domain", domain))

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("生成私钥失败: %v", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return "", "", fmt.Errorf("生成序列号失败: %v", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(10 * 365 * 24 * time.Hour) // 10年有效期

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   domain,
			Organization: []string{"LXD Proxy"},
		},
		DNSNames:              []string{domain},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", fmt.Errorf("创建证书失败: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	certPath := m.GetCertPath(domain)
	keyPath := m.GetKeyPath(domain)

	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return "", "", fmt.Errorf("写入证书文件失败: %v", err)
	}

	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		os.Remove(certPath)
		return "", "", fmt.Errorf("写入私钥文件失败: %v", err)
	}

	logger.Global.Info(ctx, "自签名证书生成成功",
		zap.String("domain", domain),
		zap.String("cert", certPath),
		zap.String("key", keyPath),
		zap.Time("valid_until", notAfter))

	return certPath, keyPath, nil
}

func (m *SSLCertManager) DeleteCert(domain string) error {
	ctx := context.Background()
	lc := &logger.Context{Action: "delete_cert"}
	ctx = logger.NewContext(ctx, lc)

	certPath := m.GetCertPath(domain)
	keyPath := m.GetKeyPath(domain)

	var errs []error

	if err := os.Remove(certPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("删除证书文件失败: %v", err))
	}

	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("删除私钥文件失败: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("删除证书时发生错误: %v", errs)
	}

	logger.Global.Info(ctx, "证书删除成功", zap.String("domain", domain))
	return nil
}

func (m *SSLCertManager) CertExists(domain string) bool {
	certPath := m.GetCertPath(domain)
	keyPath := m.GetKeyPath(domain)

	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)

	return certErr == nil && keyErr == nil
}

func (m *SSLCertManager) UpdateCert(domain, certContent, keyContent string, sslType string) (string, string, error) {
	ctx := context.Background()
	lc := &logger.Context{Action: "update_cert"}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Info(ctx, "更新证书", zap.String("domain", domain), zap.String("type", sslType))

	if m.CertExists(domain) {
		if err := m.DeleteCert(domain); err != nil {
			logger.Global.Warn(ctx, "删除旧证书失败", zap.Error(err))
		}
	}

	if sslType == "self-signed" {
		return m.GenerateSelfSignedCert(domain)
	} else if sslType == "custom" {
		return m.SaveCustomCert(domain, certContent, keyContent)
	}

	return "", "", fmt.Errorf("未知的证书类型: %s", sslType)
}

