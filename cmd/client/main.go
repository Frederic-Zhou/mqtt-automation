package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"mq_adb/pkg/models"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

// Client 手机端客户端
type Client struct {
	serialNo      string
	mqttClient    MQTT.Client
	commandTopic  string
	responseTopic string
}

// NewClient 创建新的客户端
func NewClient() (*Client, error) {
	serialNo, err := getSerialNo()
	if err != nil || serialNo == "" {
		return nil, fmt.Errorf("无法获取设备序列号: %v", err)
	}

	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		broker = "localhost"
	}
	port := os.Getenv("MQTT_PORT")
	if port == "" {
		port = "1883"
	}
	username := os.Getenv("MQTT_USERNAME")
	password := os.Getenv("MQTT_PASSWORD")

	commandTopic := fmt.Sprintf("device/no_%s/command", serialNo)
	responseTopic := fmt.Sprintf("device/no_%s/response", serialNo)

	opts := MQTT.NewClientOptions().AddBroker(fmt.Sprintf("tcp://%s:%s", broker, port))
	opts.SetClientID(fmt.Sprintf("device_%s_%d", serialNo, time.Now().Unix()))

	if username != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}

	// 设置MQTT选项
	opts.SetClientID(serialNo)           // 使用设备序列号作为客户端ID
	opts.SetCleanSession(true)           // 确保干净的会话
	opts.SetAutoReconnect(true)          // 自动重连
	opts.SetKeepAlive(60 * time.Second)  // 保持连接
	opts.SetPingTimeout(1 * time.Second) // ping超时时间

	client := &Client{
		serialNo:      serialNo,
		commandTopic:  commandTopic,
		responseTopic: responseTopic,
	}

	client.mqttClient = MQTT.NewClient(opts)

	return client, nil
}

// getSerialNo 获取设备序列号
func getSerialNo() (string, error) {
	// 检查是否有模拟序列号（用于测试）
	if mockSerial := os.Getenv("MOCK_SERIAL"); mockSerial != "" {
		return mockSerial, nil
	}

	cmd := exec.Command("adb", "shell", "getprop", "ro.serialno")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// Connect 连接到MQTT服务器
func (c *Client) Connect() error {
	if token := c.mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("连接失败: %v", token.Error())
	}

	log.Printf("设备 %s 已连接到MQTT服务器", c.serialNo)

	// 订阅命令主题
	if token := c.mqttClient.Subscribe(c.commandTopic, 0, c.handleCommand); token.Wait() && token.Error() != nil {
		return fmt.Errorf("订阅失败: %v", token.Error())
	}

	log.Printf("已订阅命令主题: %s", c.commandTopic)
	return nil
}

// handleCommand 处理收到的命令
func (c *Client) handleCommand(client MQTT.Client, msg MQTT.Message) {
	var command models.Command
	if err := json.Unmarshal(msg.Payload(), &command); err != nil {
		log.Printf("解析命令失败: %v", err)
		return
	}

	log.Printf("收到命令: %s (ID: %s)", command.Command, command.ID)

	// 执行命令并发送响应
	response := c.executeCommand(&command)
	c.sendResponse(response)
}

// executeCommand 执行命令
func (c *Client) executeCommand(command *models.Command) *models.Response {
	startTime := time.Now()

	response := &models.Response{
		ID:        command.ID,
		Status:    "success",
		Timestamp: time.Now().Unix(),
	}

	// 设置超时
	timeout := time.Duration(command.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second // 默认30秒超时
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 解析命令，支持参数
	cmdParts := strings.Fields(command.Command)
	if len(cmdParts) == 0 {
		response.Status = "error"
		response.Error = "命令不能为空"
		response.Duration = time.Since(startTime).Milliseconds()
		return response
	}

	// 执行命令 - 支持任意命令，不限于adb
	var cmd *exec.Cmd
	if len(cmdParts) == 1 {
		cmd = exec.CommandContext(ctx, cmdParts[0])
	} else {
		cmd = exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
	}

	output, err := cmd.CombinedOutput()

	response.Output = string(output)
	response.Duration = time.Since(startTime).Milliseconds()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			response.Status = "timeout"
			response.Error = "命令执行超时"
		} else {
			response.Status = "error"
			response.Error = err.Error()
		}
	}

	log.Printf("命令执行完成: %s, 状态: %s, 耗时: %dms", command.ID, response.Status, response.Duration)

	return response
}

// sendResponse 发送响应
func (c *Client) sendResponse(response *models.Response) {
	payload, err := json.Marshal(response)
	if err != nil {
		log.Printf("序列化响应失败: %v", err)
		return
	}

	token := c.mqttClient.Publish(c.responseTopic, 0, false, payload)
	if token.Wait() && token.Error() != nil {
		log.Printf("发送响应失败: %v", token.Error())
		return
	}

	log.Printf("已发送响应: %s", response.ID)
}

// Disconnect 断开连接
func (c *Client) Disconnect() {
	c.mqttClient.Disconnect(250)
	log.Println("已断开MQTT连接")
}

func main() {
	client, err := NewClient()
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}

	if err := client.Connect(); err != nil {
		log.Fatalf("连接失败: %v", err)
	}

	log.Printf("客户端 %s 已启动，等待命令...", client.serialNo)

	// 优雅退出
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("正在断开连接...")
	client.Disconnect()
}
