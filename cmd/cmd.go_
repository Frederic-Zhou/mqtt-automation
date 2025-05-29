package cmd

import (
	"encoding/json"
	"fmt"
	"mq_adb/pkg/models"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type Worker struct {
	DeviceID string
	Client   MQTT.Client
}

func (w *Worker) PublishCommand(cmd models.Command, timeout time.Duration) (string, error) {
	commandTopic := "device/" + w.DeviceID + "/command"

	// 发布命令到指定topic
	payload, err := json.Marshal(cmd)
	if err != nil {
		return "", err
	}

	token := w.Client.Publish(commandTopic, 0, false, payload)
	token.Wait()
	if token.Error() != nil {
		return "", token.Error()
	}

	// --- wait for response ---
	responseTopic := "device/" + w.DeviceID + "/response"
	responseChan := make(chan models.Response, 1)

	responseHandler := func(_ MQTT.Client, msg MQTT.Message) {
		var resp models.Response
		if err := json.Unmarshal(msg.Payload(), &resp); err == nil {
			// 订阅的就是专属 topic，无需再次检查 device_id
			responseChan <- resp
		}
	}

	if token := w.Client.Subscribe(responseTopic, 0, responseHandler); token.Wait() && token.Error() != nil {
		return "", token.Error()
	}
	defer w.Client.Unsubscribe(responseTopic)

	select {
	case resp := <-responseChan:
		if resp.Status == "success" {
			return resp.Result, nil
		}
		return "", fmt.Errorf("command execution failed: %s", resp.Error)

	case <-time.After(timeout):
		return "", fmt.Errorf("timeout waiting for response on %s", responseTopic)
	}
}
