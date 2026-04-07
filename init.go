package main

import (
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// 全局变量
var config Config
var configName string = "config.yaml"

var db *sql.DB
var dbMutex sync.RWMutex

var dirPath string

var err error

var mechineIPs MechineIPs

var responsebodyStorePath string

//go:embed websrc/go-catch-webpanel/dist/*
var webpanel embed.FS

func initCatch() bool {
	dirPath, err = os.Getwd()
	if err != nil {
		fmt.Printf("无法获取当前目录: %v\n", err)
		return false
	}
	checkMechineIPAddress()

	err := initDB()
	if err != nil {
		fmt.Printf("数据库加载失败：%v\n", err)
		return false
	}
	fmt.Println("SQLite 数据库加载成功")
	err = LoadConfig(filepath.Join(dirPath, configName))
	if err != nil {
		fmt.Println(err)
		return false
	}
	fmt.Println("配置加载成功")
	if !checkCertStatus() {
		return false
	}
	fmt.Println("证书状态检查通过")

	err = checkResponseStoreDir()
	if err != nil {
		fmt.Println("WebPanel 无法启动")
		fmt.Println(err)
		return false
	}
	fmt.Println("WebPanel 初始化成功")

	return true
}

func checkMechineIPAddress() {
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Printf("无法获取网络接口: %v\n", err)
		return
	}
	var addrspool []net.Addr
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			fmt.Printf("无法获取接口地址: %v\n", err)
			continue
		}
		addrspool = append(addrspool, addrs...)
	}
	for _, addr := range addrspool {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}

		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
			continue
		}

		if ip.IsLoopback() {
			mechineIPs.Loopback = append(mechineIPs.Loopback, ip.String())
		} else if ip.IsPrivate() {
			mechineIPs.Private = append(mechineIPs.Private, ip.String())
		} else {
			mechineIPs.Public = append(mechineIPs.Public, ip.String())
		}
	}
}

func checkResponseStoreDir() error {
	responsebodyStorePath = filepath.Join(dirPath, "responsebody")
	if _, err := os.Stat(responsebodyStorePath); os.IsNotExist(err) {
		err = os.MkdirAll(responsebodyStorePath, 0755)
		if err != nil {
			return fmt.Errorf("无法创建响应体存储目录: %v", err)
		}
	}
	return nil
}

func LoadConfig(path string) error {
	// 打开配置文件
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("配置文件 %s 不存在，正在生成默认配置...\n", path)
		err = generateConfig()
		if err != nil {
			return fmt.Errorf("生成默认配置失败: %v", err)
		}
		fmt.Printf("默认配置已生成: %s\n", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("无法打开配置文件: %v", err)
	}
	defer file.Close()

	// 解析 YAML 配置
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return fmt.Errorf("无法解析配置文件: %v", err)
	}

	return nil
}

func generateConfig() error {
	randomBytes := make([]byte, 16) // 确保 crypto/rand 包被引入以生成随机串
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Errorf("生成随机串失败: %v", err)
	}
	randomStr := hex.EncodeToString(randomBytes)

	defaultConfig := Config{
		Proxy: ProxyConfig{
			HttpPort:  8080,
			PrivateIP: true,
			PublicIP:  false,
		},
		WebPanel: WebPanelConfig{
			Port:      61233,
			PrivateIP: true,
			PublicIP:  false,
		},
		SSL: SSLConfig{
			Path: randomStr,
			Cert: "go-catch-ca.crt",
			Key:  "go-catch-ca.key",
		},
	}
	data, err := yaml.Marshal(&defaultConfig)
	if err != nil {
		return fmt.Errorf("无法生成默认配置: %v", err)
	}
	err = os.WriteFile(filepath.Join(dirPath, configName), data, 0644)
	if err != nil {
		return fmt.Errorf("无法写入默认配置文件: %v", err)
	}

	return nil
}
