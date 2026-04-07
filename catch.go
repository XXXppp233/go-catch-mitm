package main

import "fmt"

func main() {
	result := initCatch()
	if !result {
		fmt.Println("初始化失败")
		return
	}

	runHTTPSProxy()
	runWebPanel()
	select {}
}
