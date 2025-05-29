package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

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

	opts := MQTT.NewClientOptions().AddBroker(fmt.Sprintf("tcp://%s:%s", broker, port))
	opts.SetClientID(fmt.Sprintf("server_%d", time.Now().Unix()))

	if username != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}

	client := &Client{}

	opts.SetDefaultPublishHandler(func(client MQTT.Client, msg MQTT.Message) {
		log.Printf("Received message: %s from topic: %s", msg.Payload(), msg.Topic())
	})

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
	topic := "device/+/response"
	token := c.client.Subscribe(topic, 0, func(client MQTT.Client, msg MQTT.Message) {
		log.Printf("Raw MQTT message received on topic %s: %s", msg.Topic(), string(msg.Payload()))

		var response models.Response
		if err := json.Unmarshal(msg.Payload(), &response); err != nil {
			log.Printf("Failed to unmarshal response: %v", err)
			return
		}

		log.Printf("Parsed response - ID: %s, Status: %s", response.ID, response.Status)

		if c.responseHandler != nil {
			c.responseHandler(&response)
		} else {
			log.Printf("No response handler set")
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
