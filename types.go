package main

import "time"

// 按照 abcdef 的顺序排列

type Config struct {
	Proxy    ProxyConfig    `yaml:"Proxy"`
	WebPanel WebPanelConfig `yaml:"WebPanel"`
	SSL      SSLConfig      `yaml:"SSL"`
}

type MechineIPs struct {
	Loopback []string
	Private  []string
	Public   []string
}

type ProxyConfig struct {
	HttpPort  int  `yaml:"HttpPort"`
	PrivateIP bool `yaml:"PrivateIP"`
	PublicIP  bool `yaml:"PublicIP"`
}

type reqInfo struct {
	start      time.Time
	srcIP      string
	method     string
	url        string
	dstHost    string
	dstIP      string
	proto      string
	userAgent  string
	reqHeaders map[string][]string
	reqBody    []byte
}

type SSLConfig struct {
	Path string `yaml:"Path"`
	Cert string `yaml:"Cert"`
	Key  string `yaml:"Key"`
}

type trafficRecord struct {
	CreatedAt    string `json:"created_at"`
	Method       string `json:"method"`
	SrcIP        string `json:"src_ip"`
	DstHost      string `json:"dst_host"`
	DstIP        string `json:"dst_ip"`
	URL          string `json:"url"`
	Status       int    `json:"status"`
	ReqHeaders   string `json:"req_headers"`
	ReqBody      []byte `json:"req_body"`
	RespHeaders  string `json:"resp_headers"`
	RespBodyPath string `json:"resp_body_path"`
	UserAgent    string `json:"user_agent"`
}

type WebPanelConfig struct {
	Port      int  `yaml:"Port"`
	PrivateIP bool `yaml:"PrivateIP"`
	PublicIP  bool `yaml:"PublicIP"`
}
