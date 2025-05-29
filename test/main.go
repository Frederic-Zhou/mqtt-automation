package main

import (
	"fmt"
	"mq_adb/cmd"
	"mq_adb/scripts"
	"os"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

func main() {
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
	if username != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}

	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println("MQTT连接失败:", token.Error())
		return
	}
	fmt.Println("MQTT服务器已连接")

	scripts.My_first(&cmd.Worker{
		DeviceID: "no_123456", // 示例设备ID
		Client:   client,
	})
}
