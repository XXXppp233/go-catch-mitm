package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

func runWebPanel() {
	webFS, err := fs.Sub(webpanel, "websrc/go-catch-webpanel/dist")
	if err != nil {
		fmt.Printf("无法访问 WebPanel 静态资源: %v\n", err)
		return
	}
	// 静态资源
	http.Handle("/", http.FileServer(http.FS(webFS)))
	// response body 静态文件
	http.Handle("/responsebody/", http.StripPrefix("/responsebody/", http.FileServer(http.Dir(responsebodyStorePath))))
	// API 路由
	http.HandleFunc("/api/", handleAPI)
	// 证书下载路由
	http.HandleFunc(fmt.Sprintf("/%s/", config.SSL.Path), HandleCertDownload)

	// loopback IP 监听
	for _, ip := range mechineIPs.Loopback {
		go func(ip string) {
			if strings.Contains(ip, ":") {
				ip = fmt.Sprintf("[%s]", ip) // IPv6 地址需要加方括号
			}
			fmt.Printf("WebPanel 监听在回环 IP: http://%s:%d\n", ip, config.WebPanel.Port)
			err = http.ListenAndServe(fmt.Sprintf("%s:%d", ip, config.WebPanel.Port), nil)
			if err != nil {
				fmt.Printf("WebPanel 监听失败: %v\n", err)
			}
		}(ip)
	}
	// 私有 IP 监听

	if config.WebPanel.PrivateIP {
		for _, ip := range mechineIPs.Private {
			go func(ip string) {
				if strings.Contains(ip, ":") {
					ip = fmt.Sprintf("[%s]", ip) // IPv6 地址需要加方括号
				}
				fmt.Printf("WebPanel 监听在私有 IP: http://%s:%d\n", ip, config.WebPanel.Port)
				fmt.Printf("WebPanel 证书下载地址: http://%s:%d/%s/%s\n", ip, config.WebPanel.Port, config.SSL.Path, config.SSL.Cert)
				err = http.ListenAndServe(fmt.Sprintf("%s:%d", ip, config.WebPanel.Port), nil)
				if err != nil {
					fmt.Printf("WebPanel 监听失败: %v\n", err)
				}
			}(ip)
		}
	}
	// 公共 IP 监听
	if config.WebPanel.PublicIP {
		for _, ip := range mechineIPs.Public {
			go func(ip string) {
				if strings.Contains(ip, ":") {
					ip = fmt.Sprintf("[%s]", ip) // IPv6 地址需要加方括号
				}
				fmt.Printf("WebPanel 监听在公共 IP: http://%s:%d\n", ip, config.WebPanel.Port)
				fmt.Printf("WebPanel 证书下载地址: http://%s:%d/%s/%s\n", ip, config.WebPanel.Port, config.SSL.Path, config.SSL.Cert)
				err = http.ListenAndServe(fmt.Sprintf("%s:%d", ip, config.WebPanel.Port), nil)
				if err != nil {
					fmt.Printf("WebPanel 监听失败: %v\n", err)
				}
			}(ip)
		}
	}
	// select {} // 阻塞主线程

}

func handleAPI(w http.ResponseWriter, r *http.Request) {
	// 这里可以根据 URL 路径来区分不同的 API 请求
	switch r.URL.Path {
	case "/api/traffic":
		handleTrafficAPI(w, r)
	default:
		http.NotFound(w, r)
	}
}

func handleTrafficAPI(w http.ResponseWriter, r *http.Request) {
	// 或许应该添加默认分页参数以避免一次性查询过多数据
	list, err := queryTrafficRecords()

	if err != nil {
		http.Error(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	if err = enc.Encode(list); err != nil {
		http.Error(w, "encode failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func HandleCertDownload(w http.ResponseWriter, r *http.Request) {
	certPath := filepath.Join(dirPath, config.SSL.Cert)
	keyPath := filepath.Join(dirPath, config.SSL.Key)

	// 处理证书下载请求
	if r.URL.Path == fmt.Sprintf("/%s/%s", config.SSL.Path, config.SSL.Cert) {
		http.ServeFile(w, r, certPath)
		return
	} else if r.URL.Path == fmt.Sprintf("/%s/%s", config.SSL.Path, config.SSL.Key) {
		http.ServeFile(w, r, keyPath)
		return
	}

	http.NotFound(w, r)
}
