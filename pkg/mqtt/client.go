package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"mq_adb/pkg/models"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

// Client MQTT客户端
type Client struct {
	mqttClient MQTT.Client
	responses  map[string]*models.Response
	mutex      sync.RWMutex
	timeout    time.Duration
}

// NewClient 创建MQTT客户端
func NewClient() (*Client, error) {
	// MQTT配置
	broker := "localhost"
	port := "1883"
	username := "user1"
	password := "123456"

	opts := MQTT.NewClientOptions().AddBroker(fmt.Sprintf("tcp://%s:%s", broker, port))
	opts.SetClientID(fmt.Sprintf("server_%d", time.Now().Unix()))
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(1 * time.Second)

	client := &Client{
		responses: make(map[string]*models.Response),
		timeout:   30 * time.Second,
	}

	opts.SetDefaultPublishHandler(client.messageHandler)

	client.mqttClient = MQTT.NewClient(opts)

	// 连接到MQTT服务器
	if token := client.mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("MQTT连接失败: %v", token.Error())
	}

	log.Println("MQTT客户端已连接到服务器")

	// 订阅所有设备的响应主题 (使用通配符)
	if token := client.mqttClient.Subscribe("device/+/response", 0, client.responseHandler); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("订阅响应主题失败: %v", token.Error())
	}

	log.Println("已订阅设备响应主题")

	return client, nil
}

// messageHandler 默认消息处理器
func (c *Client) messageHandler(client MQTT.Client, msg MQTT.Message) {
	log.Printf("收到未处理的消息: %s", msg.Topic())
}

// responseHandler 响应处理器
func (c *Client) responseHandler(client MQTT.Client, msg MQTT.Message) {
	var response models.Response
	if err := json.Unmarshal(msg.Payload(), &response); err != nil {
		log.Printf("解析响应失败: %v", err)
		return
	}

	log.Printf("收到设备响应: ID=%s, Status=%s", response.ID, response.Status)

	// 保存响应
	c.mutex.Lock()
	c.responses[response.ID] = &response
	c.mutex.Unlock()
}

// ExecuteCommand 执行命令
func (c *Client) ExecuteCommand(cmd *models.Command) (*models.Response, error) {
	// 发送命令
	commandTopic := fmt.Sprintf("device/no_%s/command", cmd.SerialNo)
	payload, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("序列化命令失败: %v", err)
	}

	log.Printf("发送命令到设备 %s: %s", cmd.SerialNo, cmd.Command)

	token := c.mqttClient.Publish(commandTopic, 0, false, payload)
	if token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("发送命令失败: %v", token.Error())
	}

	// 等待响应
	timeout := time.Duration(cmd.Timeout) * time.Second
	if timeout == 0 {
		timeout = c.timeout
	}

	startTime := time.Now()
	for {
		// 检查是否收到响应
		c.mutex.RLock()
		response, exists := c.responses[cmd.ID]
		c.mutex.RUnlock()

		if exists {
			// 清理响应记录
			c.mutex.Lock()
			delete(c.responses, cmd.ID)
			c.mutex.Unlock()

			log.Printf("命令执行完成: ID=%s, Status=%s, Duration=%dms",
				response.ID, response.Status, response.Duration)
			return response, nil
		}

		// 检查超时
		if time.Since(startTime) > timeout {
			return &models.Response{
				ID:        cmd.ID,
				Status:    "timeout",
				Error:     "等待设备响应超时",
				Timestamp: time.Now().Unix(),
				Duration:  time.Since(startTime).Milliseconds(),
			}, nil
		}

		// 短暂等待
		time.Sleep(100 * time.Millisecond)
	}
}

// SetTimeout 设置超时时间
func (c *Client) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// Disconnect 断开连接
func (c *Client) Disconnect() {
	if c.mqttClient != nil && c.mqttClient.IsConnected() {
		c.mqttClient.Disconnect(250)
		log.Println("MQTT客户端已断开连接")
	}
}

// GetPendingResponses 获取待处理的响应数量（用于调试）
func (c *Client) GetPendingResponses() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.responses)
}
