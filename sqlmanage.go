package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

func initDB() error {
	dbPath := filepath.Join(dirPath, "traffic.db")

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %v", err)
	}
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS traffic (
		created_at TEXT NOT NULL,
		method TEXT,
		src_ip TEXT,
		dst_host TEXT,
		dst_ip TEXT,
		url TEXT,
		status INTEGER,
		req_headers TEXT,
		req_body BLOB,
		resp_headers TEXT,
		resp_body_path TEXT PRIMARY KEY,
		user_agent TEXT
	);
	`)
	if err != nil {
		return fmt.Errorf("创建表失败: %v", err)
	}
	return nil
}

func storeResponseBody(body []byte) (string, error) {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("生成随机串失败: %v", err)
	}
	randomStr := hex.EncodeToString(randomBytes)

	filePath := filepath.Join(responsebodyStorePath, randomStr)
	if err := os.WriteFile(filePath, body, 0644); err != nil {
		return "", fmt.Errorf("写入 response body 失败: %v", err)
	}

	// 数据库中保存可访问路径
	return randomStr, nil
}

func saveTrafficRecord(info *reqInfo, reqHeaders map[string][]string, reqBody []byte, status int, respHeaders map[string][]string, respBody []byte) error {

	dbMutex.Lock()
	defer dbMutex.Unlock()

	reqHeadersJSON, _ := json.Marshal(reqHeaders)
	respHeadersJSON, _ := json.Marshal(respHeaders)

	respBodyPath, err := storeResponseBody(respBody)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
INSERT INTO traffic (
	created_at, method, src_ip, dst_host, dst_ip, url, status,
	req_headers, req_body, resp_headers, resp_body_path, user_agent
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
`,
		time.Now().Format(time.RFC3339),
		info.method,
		info.srcIP,
		info.dstHost,
		info.dstIP,
		info.url,
		status,
		string(reqHeadersJSON),
		reqBody,
		string(respHeadersJSON),
		respBodyPath,
		info.userAgent,
	)
	if err != nil {
		return fmt.Errorf("写入数据库失败: %v", err)
	}
	return nil
}

func queryTrafficRecords() ([]trafficRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	dbMutex.RLock()
	defer dbMutex.RUnlock()

	rows, err := db.Query(`
SELECT created_at, method, src_ip, dst_host, dst_ip, url, status,
       req_headers, req_body, resp_headers, resp_body_path, user_agent
FROM traffic
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []trafficRecord
	for rows.Next() {
		var rec trafficRecord
		if err := rows.Scan(
			&rec.CreatedAt,
			&rec.Method,
			&rec.SrcIP,
			&rec.DstHost,
			&rec.DstIP,
			&rec.URL,
			&rec.Status,
			&rec.ReqHeaders,
			&rec.ReqBody,
			&rec.RespHeaders,
			&rec.RespBodyPath,
			&rec.UserAgent,
		); err != nil {
			fmt.Println(err)
			return nil, err
		}
		list = append(list, rec)
	}
	return list, nil
}
