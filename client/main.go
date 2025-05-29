package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"mq_adb/pkg/models"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

// Client 手机端客户端
type Client struct {
	deviceID      string
	mqttClient    MQTT.Client
	commandTopic  string
	responseTopic string
}

// NewClient 创建新的客户端
func NewClient() (*Client, error) {
	deviceID, err := getDeviceID()
	if err != nil || deviceID == "" {
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

	commandTopic := fmt.Sprintf("device/%s/command", deviceID)
	responseTopic := fmt.Sprintf("device/%s/response", deviceID)

	opts := MQTT.NewClientOptions().AddBroker(fmt.Sprintf("tcp://%s:%s", broker, port))
	opts.SetClientID(fmt.Sprintf("device_%s_%d", deviceID, time.Now().Unix()))

	if username != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}

	client := &Client{
		deviceID:      deviceID,
		commandTopic:  commandTopic,
		responseTopic: responseTopic,
	}

	client.mqttClient = MQTT.NewClient(opts)

	return client, nil
}

// getDeviceID 获取设备序列号
func getDeviceID() (string, error) {
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

	log.Printf("设备 %s 已连接到MQTT服务器", c.deviceID)

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

	log.Printf("收到命令: %s (ID: %s)", command.Type, command.ID)

	startTime := time.Now()
	response := c.executeCommand(&command)
	response.Duration = time.Since(startTime).Milliseconds()

	// 发送响应
	c.sendResponse(response)
}

// executeCommand 执行命令
func (c *Client) executeCommand(command *models.Command) *models.Response {
	response := &models.Response{
		ID:        command.ID,
		Command:   command.Command,
		Status:    "success",
		Timestamp: time.Now().Unix(),
	}

	switch command.Type {
	case "shell":
		c.executeShellCommand(command, response)
	case "tap":
		c.executeTapCommand(command, response)
	case "input":
		c.executeInputCommand(command, response)
	case "screenshot":
		c.executeScreenshotCommand(command, response)
	case "check_text":
		c.executeCheckTextCommand(command, response)
	case "tap_text":
		c.executeTapTextCommand(command, response)
	case "wait":
		c.executeWaitCommand(command, response)
	default:
		response.Status = "error"
		response.Error = fmt.Sprintf("未知命令类型: %s", command.Type)
	}

	return response
}

// executeShellCommand 执行Shell命令
func (c *Client) executeShellCommand(command *models.Command, response *models.Response) {
	if command.Command == "" {
		response.Status = "error"
		response.Error = "命令为空"
		return
	}

	// 模拟模式处理
	if os.Getenv("MOCK_SERIAL") != "" {
		response.Result = c.simulateShellCommand(command.Command, command.Args)
		return
	}

	args := command.Args
	if len(args) == 0 {
		// 如果没有参数，使用shell执行
		args = []string{"-c", command.Command}
		command.Command = "/bin/sh"
	}

	cmd := exec.Command(command.Command, args...)
	output, err := cmd.CombinedOutput()

	response.Result = string(output)
	if err != nil {
		response.Status = "error"
		response.Error = err.Error()
	}
}

// executeTapCommand 执行点击命令
func (c *Client) executeTapCommand(command *models.Command, response *models.Response) {
	if command.X <= 0 || command.Y <= 0 {
		response.Status = "error"
		response.Error = "无效的坐标"
		return
	}

	cmd := exec.Command("adb", "shell", "input", "tap",
		strconv.Itoa(command.X), strconv.Itoa(command.Y))
	output, err := cmd.CombinedOutput()

	response.Result = string(output)
	if err != nil {
		response.Status = "error"
		response.Error = err.Error()
	}
}

// executeInputCommand 执行输入命令
func (c *Client) executeInputCommand(command *models.Command, response *models.Response) {
	if command.Text == "" {
		response.Status = "error"
		response.Error = "输入文本为空"
		return
	}

	cmd := exec.Command("adb", "shell", "input", "text", command.Text)
	output, err := cmd.CombinedOutput()

	response.Result = string(output)
	if err != nil {
		response.Status = "error"
		response.Error = err.Error()
	}
}

// executeScreenshotCommand 执行截图命令
func (c *Client) executeScreenshotCommand(command *models.Command, response *models.Response) {
	// 截图并保存
	screenshotPath := "/sdcard/screenshot.png"
	cmd := exec.Command("adb", "shell", "screencap", "-p", screenshotPath)
	if err := cmd.Run(); err != nil {
		response.Status = "error"
		response.Error = fmt.Sprintf("截图失败: %v", err)
		return
	}

	// 获取截图文件
	cmd = exec.Command("adb", "pull", screenshotPath, "./screenshot.png")
	if err := cmd.Run(); err != nil {
		response.Status = "error"
		response.Error = fmt.Sprintf("获取截图失败: %v", err)
		return
	}

	// 获取屏幕文本信息（使用uiautomator dump）
	textInfo, err := c.getScreenTextInfo()
	if err != nil {
		log.Printf("获取屏幕文本信息失败: %v", err)
	} else {
		response.TextInfo = textInfo
	}

	response.Result = "截图完成"
	response.Screenshot = "screenshot.png" // 实际项目中可以返回base64编码的图片
}

// executeCheckTextCommand 检查文本是否存在
func (c *Client) executeCheckTextCommand(command *models.Command, response *models.Response) {
	textInfo, err := c.getScreenTextInfo()
	if err != nil {
		response.Status = "error"
		response.Error = fmt.Sprintf("获取屏幕信息失败: %v", err)
		return
	}

	found := false
	for _, info := range textInfo {
		if strings.Contains(info.Text, command.Text) {
			found = true
			response.Result = fmt.Sprintf("找到文本 '%s' 在坐标 (%d, %d)", command.Text, info.X, info.Y)
			break
		}
	}

	if !found {
		response.Status = "error"
		response.Error = fmt.Sprintf("未找到文本: %s", command.Text)
	}

	response.TextInfo = textInfo
}

// executeTapTextCommand 点击包含指定文本的元素
func (c *Client) executeTapTextCommand(command *models.Command, response *models.Response) {
	textInfo, err := c.getScreenTextInfo()
	if err != nil {
		response.Status = "error"
		response.Error = fmt.Sprintf("获取屏幕信息失败: %v", err)
		return
	}

	for _, info := range textInfo {
		if strings.Contains(info.Text, command.Text) {
			// 计算点击位置（元素中心）
			clickX := info.X + info.Width/2
			clickY := info.Y + info.Height/2

			cmd := exec.Command("adb", "shell", "input", "tap",
				strconv.Itoa(clickX), strconv.Itoa(clickY))
			_, err := cmd.CombinedOutput()

			if err != nil {
				response.Status = "error"
				response.Error = err.Error()
			} else {
				response.Result = fmt.Sprintf("点击了文本 '%s' 在坐标 (%d, %d)", command.Text, clickX, clickY)
			}
			return
		}
	}

	response.Status = "error"
	response.Error = fmt.Sprintf("未找到包含文本 '%s' 的元素", command.Text)
}

// executeWaitCommand 执行等待命令
func (c *Client) executeWaitCommand(command *models.Command, response *models.Response) {
	waitTime := 1 // 默认等待1秒
	if command.Timeout > 0 {
		waitTime = command.Timeout
	}

	time.Sleep(time.Duration(waitTime) * time.Second)
	response.Result = fmt.Sprintf("等待了 %d 秒", waitTime)
}

// getScreenTextInfo 获取屏幕文本信息
func (c *Client) getScreenTextInfo() ([]models.TextPosition, error) {
	// 使用uiautomator dump获取UI信息
	cmd := exec.Command("adb", "shell", "uiautomator", "dump", "/sdcard/ui.xml")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("UI dump失败: %v", err)
	}

	// 获取XML文件
	cmd = exec.Command("adb", "shell", "cat", "/sdcard/ui.xml")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("读取UI信息失败: %v", err)
	}

	// 解析XML并提取文本位置信息
	// 这里简化处理，实际项目中需要完整的XML解析
	textPositions := []models.TextPosition{}

	// 简单的文本提取示例
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "text=") && strings.Contains(line, "bounds=") {
			text := extractText(line)
			x, y, width, height := extractBounds(line)
			if text != "" {
				textPositions = append(textPositions, models.TextPosition{
					Text:   text,
					X:      x,
					Y:      y,
					Width:  width,
					Height: height,
				})
			}
		}
	}

	return textPositions, nil
}

// extractText 从XML行中提取文本
func extractText(line string) string {
	start := strings.Index(line, `text="`)
	if start == -1 {
		return ""
	}
	start += 6
	end := strings.Index(line[start:], `"`)
	if end == -1 {
		return ""
	}
	return line[start : start+end]
}

// extractBounds 从XML行中提取坐标信息
func extractBounds(line string) (x, y, width, height int) {
	start := strings.Index(line, `bounds="[`)
	if start == -1 {
		return 0, 0, 0, 0
	}
	start += 9
	end := strings.Index(line[start:], `]"`)
	if end == -1 {
		return 0, 0, 0, 0
	}

	bounds := line[start : start+end]
	coords := strings.Split(bounds, "][")
	if len(coords) != 2 {
		return 0, 0, 0, 0
	}

	// 解析第一个坐标 [x1,y1]
	coord1 := strings.Split(coords[0], ",")
	if len(coord1) != 2 {
		return 0, 0, 0, 0
	}

	// 解析第二个坐标 [x2,y2]
	coord2 := strings.Split(coords[1], ",")
	if len(coord2) != 2 {
		return 0, 0, 0, 0
	}

	x1, _ := strconv.Atoi(coord1[0])
	y1, _ := strconv.Atoi(coord1[1])
	x2, _ := strconv.Atoi(coord2[0])
	y2, _ := strconv.Atoi(coord2[1])

	return x1, y1, x2 - x1, y2 - y1
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

	log.Printf("设备 %s 已启动，等待命令...", client.deviceID)

	// 优雅退出
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("正在断开连接...")
	client.Disconnect()
}

// simulateShellCommand 模拟Shell命令执行（用于测试）
func (c *Client) simulateShellCommand(command string, args []string) string {
	fullCmd := command
	if len(args) > 0 {
		fullCmd += " " + strings.Join(args, " ")
	}

	// 模拟一些常见的系统命令
	switch {
	case strings.Contains(command, "getprop ro.product.model"):
		return "TEST_DEVICE_MODEL"
	case strings.Contains(command, "getprop ro.build.version.release"):
		return "11"
	case strings.Contains(command, "wm size"):
		return "Physical size: 1080x2400"
	case strings.Contains(command, "wm density"):
		return "Physical density: 440"
	case strings.Contains(command, "cat /proc/meminfo"):
		return "MemTotal:        8000000 kB"
	case strings.Contains(command, "dumpsys battery"):
		return "level: 85"
	case strings.Contains(command, "dumpsys wifi"):
		return "Wi-Fi is enabled"
	case strings.Contains(command, "ping"):
		return "PING www.baidu.com (14.215.177.38): 56 data bytes\n64 bytes from 14.215.177.38: icmp_seq=0 ttl=56 time=12.345 ms"
	case strings.Contains(command, "ip addr show wlan0"):
		return "inet 192.168.1.100/24 brd 192.168.1.255 scope global wlan0"
	default:
		return fmt.Sprintf("模拟执行命令: %s\n命令输出: 模拟结果", fullCmd)
	}
}
