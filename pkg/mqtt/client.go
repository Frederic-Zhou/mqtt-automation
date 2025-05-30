package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"mq_adb/pkg/config"
	"mq_adb/pkg/models"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

// Client MQTT客户端封装
type Client struct {
	client          MQTT.Client
	responseHandler func(*models.Response)
}

// NewClient 创建新的MQTT客户端
func NewClient() *Client {
	cfg := config.LoadConfig()

	brokerURL := fmt.Sprintf("tcp://%s:%s", cfg.MQTTBroker, cfg.MQTTPort)

	opts := MQTT.NewClientOptions().AddBroker(brokerURL)
	opts.SetClientID(fmt.Sprintf("server_%d", time.Now().Unix()))

	if cfg.MQTTUsername != "" {
		opts.SetUsername(cfg.MQTTUsername)
		opts.SetPassword(cfg.MQTTPassword)
	}

	// 设置连接选项
	opts.SetKeepAlive(60 * time.Second)
	opts.SetDefaultPublishHandler(func(client MQTT.Client, msg MQTT.Message) {
		log.Printf("Received message: %s from topic: %s", msg.Payload(), msg.Topic())
	})

	client := &Client{}
	client.client = MQTT.NewClient(opts)

	return client
}

// Connect 连接到MQTT服务器
func (c *Client) Connect() error {
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT connection failed: %v", token.Error())
	}

	log.Println("Connected to MQTT broker")

	// 订阅所有设备的响应
	c.SubscribeResponses()

	return nil
}

// SetResponseHandler 设置响应处理器
func (c *Client) SetResponseHandler(handler func(*models.Response)) {
	c.responseHandler = handler
}

// SubscribeResponses 订阅所有设备的响应
func (c *Client) SubscribeResponses() error {
	topic := "device/no_+/response"
	token := c.client.Subscribe(topic, 0, func(client MQTT.Client, msg MQTT.Message) {
		var response models.Response
		if err := json.Unmarshal(msg.Payload(), &response); err != nil {
			log.Printf("Failed to unmarshal response: %v", err)
			return
		}

		log.Printf("Received response from device: %s", response.ID)

		if c.responseHandler != nil {
			c.responseHandler(&response)
		}
	})

	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("subscribe failed: %v", token.Error())
	}

	log.Printf("Subscribed to response topic: %s", topic)
	return nil
}

// PublishCommand 发布命令到指定设备
func (c *Client) PublishCommand(topic string, command *models.Command) error {
	payload, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("marshal command failed: %v", err)
	}

	token := c.client.Publish(topic, 0, false, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("publish failed: %v", token.Error())
	}

	log.Printf("Published command %s to topic %s", command.ID, topic)
	return nil
}

// Disconnect 断开连接
func (c *Client) Disconnect() {
	c.client.Disconnect(250)
	log.Println("Disconnected from MQTT broker")
}

// IsConnected 检查连接状态
func (c *Client) IsConnected() bool {
	return c.client.IsConnected()
}
