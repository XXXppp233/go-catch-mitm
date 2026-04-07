package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
)

func readAndRestoreBody(body io.ReadCloser) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	data, err := io.ReadAll(body)
	_ = body.Close()
	return data, err
}

// Such as "1.1.1.1:1111", "example.com:443" or "example.com"
func splitHostPort(hostport string) (string, string) {
	if hostport == "" {
		return "", ""
	}
	if strings.Contains(hostport, ":") {
		host, port, err := net.SplitHostPort(hostport)
		if err == nil {
			return host, port
		}
	}
	return hostport, ""
}

func resolveFirstIP(host string) string {
	if host == "" {
		return ""
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return ""
	}
	return ips[0].String()
}

func runHTTPSProxy() {
	proxy := goproxy.NewProxyHttpServer()
	caCert, err := tls.LoadX509KeyPair(config.SSL.Cert, config.SSL.Key)
	if err != nil {
		log.Fatalf("加载私有证书失败: %v", err)
	}
	goproxy.GoproxyCa = caCert

	// 当遇到 CONNECT 请求时，总是开启 MITM
	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	proxy.OnRequest().DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			srcHost, _, _ := net.SplitHostPort(r.RemoteAddr)
			if srcHost == "" {
				srcHost = r.RemoteAddr
			}

			host := r.Host
			if host == "" {
				host = r.URL.Host
			}
			dstHost, _ := splitHostPort(host)
			dstIP := resolveFirstIP(dstHost)

			info := &reqInfo{
				start:      time.Now(),
				srcIP:      srcHost,
				method:     r.Method,
				url:        r.URL.String(),
				dstHost:    dstHost,
				dstIP:      dstIP,
				proto:      r.Proto,
				userAgent:  r.UserAgent(),
				reqHeaders: r.Header.Clone(),
			}
			ctx.UserData = info

			// 读取请求体（并恢复）
			if r.Method != http.MethodConnect {
				bodyBytes, err := readAndRestoreBody(r.Body) // Body 转化为 []byte 以存储
				if err == nil {
					r.Body = io.NopCloser(bytes.NewReader(bodyBytes)) // []byte 恢复为 io.ReadCloser 以正常响应请求
					info.reqBody = bodyBytes
					log.Printf("[请求] %s %s | SrcIP=%s | DstHost=%s | DstIP=%s", info.method, info.url, info.srcIP, info.dstHost, info.dstIP)
				}
			}
			return r, nil
		},
	)

	proxy.OnResponse().DoFunc(
		func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			if resp == nil {
				return resp
			}
			info, _ := ctx.UserData.(*reqInfo)
			if info == nil {
				info = &reqInfo{}
			}
			elapsed := time.Since(info.start)

			bodyBytes, err := readAndRestoreBody(resp.Body)
			if err == nil {
				resp.Body = io.NopCloser(bytes.NewReader(bodyBytes)) // []byte 恢复为 io.ReadCloser 以正常响应请求
				log.Printf("[响应] %s %s | Status=%d | Cost=%s", info.method, info.url, resp.StatusCode, elapsed)

				err = saveTrafficRecord(info, info.reqHeaders, info.reqBody, resp.StatusCode, resp.Header.Clone(), bodyBytes)
				if err != nil {
					log.Printf("保存数据库失败: %v", err)
				}
			}
			return resp
		},
	)

	port := config.Proxy.HttpPort

	// loopback IP 监听
	for _, ip := range mechineIPs.Loopback {
		go func(ip string) {
			if strings.Contains(ip, ":") {
				ip = fmt.Sprintf("[%s]", ip) // IPv6 地址需要加方括号
			}
			fmt.Printf("HTTP(S) Proxy 监听在回环 IP: http://%s:%d\n", ip, port)
			err = http.ListenAndServe(fmt.Sprintf("%s:%d", ip, port), proxy)
			if err != nil {
				fmt.Printf("HTTP(S) Proxy 监听失败: %v\n", err)
			}
		}(ip)
	}

	// 私有 IP 监听
	if config.Proxy.PrivateIP {
		for _, ip := range mechineIPs.Private {
			go func(ip string) {
				if strings.Contains(ip, ":") {
					ip = fmt.Sprintf("[%s]", ip) // IPv6 地址需要加方括号
				}
				fmt.Printf("HTTP(S) Proxy 监听在私有 IP: http://%s:%d\n", ip, port)
				err = http.ListenAndServe(fmt.Sprintf("%s:%d", ip, port), proxy)
				if err != nil {
					fmt.Printf("HTTP(S) Proxy 监听失败: %v\n", err)
				}
			}(ip)
		}
	}
	// 公共 IP 监听
	if config.Proxy.PublicIP {
		for _, ip := range mechineIPs.Public {
			go func(ip string) {
				if strings.Contains(ip, ":") {
					ip = fmt.Sprintf("[%s]", ip) // IPv6 地址需要加方括号
				}
				fmt.Printf("HTTP(S) Proxy 监听在公共 IP: http://%s:%d\n", ip, port)
				err = http.ListenAndServe(fmt.Sprintf("%s:%d", ip, port), proxy)
				if err != nil {
					fmt.Printf("HTTP(S) Proxy 监听失败: %v\n", err)
				}
			}(ip)
		}
	}
}
