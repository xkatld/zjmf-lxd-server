package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"
)


func GenerateSelfSignedCert(certFile, keyFile string, hosts []string) error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("生成私钥失败: %v", err)
	}


	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"LXD API Server"},
			Country:      []string{"CN"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}


	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, host)
		}
	}


	template.IPAddresses = append(template.IPAddresses, net.IPv4(127, 0, 0, 1))
	template.DNSNames = append(template.DNSNames, "localhost")


	template.IPAddresses = append(template.IPAddresses, net.IPv4(127, 0, 0, 1))
	template.IPAddresses = append(template.IPAddresses, net.IPv6loopback)
	template.DNSNames = append(template.DNSNames, "localhost")


	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("生成证书失败: %v", err)
	}


	certOut, err := os.Create(certFile)
	if err != nil {
		return fmt.Errorf("创建证书文件失败: %v", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("写入证书文件失败: %v", err)
	}


	keyOut, err := os.Create(keyFile)
	if err != nil {
		return fmt.Errorf("创建私钥文件失败: %v", err)
	}
	defer keyOut.Close()

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("序列化私钥失败: %v", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}); err != nil {
		return fmt.Errorf("写入私钥文件失败: %v", err)
	}


	if err := os.Chmod(keyFile, 0600); err != nil {
		return fmt.Errorf("设置私钥文件权限失败: %v", err)
	}

	return nil
}


func GetServerIP() string {

	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP.String()
	}


	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return ""
}


func CertExists(certFile, keyFile string) bool {
	_, err1 := os.Stat(certFile)
	_, err2 := os.Stat(keyFile)
	return err1 == nil && err2 == nil
}


func ValidateCert(certFile string) bool {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return false
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return false
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}


	now := time.Now()
	return now.After(cert.NotBefore) && now.Before(cert.NotAfter)
}
