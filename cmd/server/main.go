package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"mq_adb/pkg/api"
)

func main() {
	log.Println("启动移动设备自动化服务端...")

	// 创建服务端
	server, err := api.NewServer()
	if err != nil {
		log.Fatalf("创建服务端失败: %v", err)
	}

	// 启动服务器
	go func() {
		log.Println("HTTP服务器启动在 :8080")
		if err := server.Run(":8080"); err != nil {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 优雅退出
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("正在关闭服务器...")
	server.Stop()
	log.Println("服务器已关闭")
}
