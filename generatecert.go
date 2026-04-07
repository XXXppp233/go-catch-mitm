package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"
)

func generateCA() error {
	// 1. 生成 RSA 私钥
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)

	// 2. 配置证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:       []string{"UK"},
			Province:      []string{"Cambridgeshire"},
			Locality:      []string{"Cambridge"},
			StreetAddress: []string{"King's Parade, Cambridge CB2 1ST, United Kingdom"},
			PostalCode:    []string{"6438+PW Cambridge, United Kingdom"},
			Organization:  []string{"Golang Catch Local Proxy CA"},
			CommonName:    "Golang Catch Trusted CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(2, 2, 3), // 2年2月3天有效期
		IsCA:                  true,                        // 标记为 CA
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	// 3. 自签名生成证书
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	// 4. 保存私钥到 pem 文件
	keyOut, _ := os.Create("go-catch-ca.key")
	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	// 5. 保存证书到 pem 文件
	certOut, _ := os.Create("go-catch-ca.crt")
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	return nil
}

// 修改逻辑，应该是客户端安装而非服务端安装，且应给出两个下载链接
func checkCertStatus() bool {
	_, err := os.ReadFile("go-catch-ca.crt")
	if err != nil {
		fmt.Println(err)
		err = generateCA()
		if err != nil {
			fmt.Printf("生成证书失败: %v\n", err)
			fmt.Println("请确保程序有权限在当前目录下创建 go-catch-ca.crt 和 go-catch-ca.key 文件")
			return false
		}
		fmt.Println("已经生成新的证书，在客户端导入证书时记得选择带有 root Trusted 等关键字")
		fmt.Println("可以继续前往 Web 面板里下载 go-catch-ca.crt ")
	}

	fmt.Println("详细信息请查看 go-catch 官方文档：https://docs.wepayto.win/application/gocatch")
	return true

}
