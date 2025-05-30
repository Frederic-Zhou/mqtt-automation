package config

import (
	"bufio"
	"os"
	"strings"
)

// Config 应用程序配置
type Config struct {
	MQTTBroker   string
	MQTTPort     string
	MQTTUsername string
	MQTTPassword string
}

// LoadConfig 从.env文件和环境变量加载配置
func LoadConfig() *Config {
	config := &Config{
		MQTTBroker:   "localhost",
		MQTTPort:     "1883",
		MQTTUsername: "",
		MQTTPassword: "",
	}

	// 先尝试从.env文件加载
	loadFromEnvFile(config)

	// 然后从环境变量覆盖（如果存在）
	if broker := os.Getenv("MQTT_BROKER"); broker != "" {
		config.MQTTBroker = broker
	}
	if port := os.Getenv("MQTT_PORT"); port != "" {
		config.MQTTPort = port
	}
	if username := os.Getenv("MQTT_USERNAME"); username != "" {
		config.MQTTUsername = username
	}
	if password := os.Getenv("MQTT_PASSWORD"); password != "" {
		config.MQTTPassword = password
	}

	return config
}

// loadFromEnvFile 从.env文件加载配置
func loadFromEnvFile(config *Config) error {
	file, err := os.Open(".env")
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// 简单去除引号
		value = strings.Trim(value, `"'`)

		switch key {
		case "MQTT_BROKER":
			config.MQTTBroker = value
		case "MQTT_PORT":
			config.MQTTPort = value
		case "MQTT_USERNAME":
			config.MQTTUsername = value
		case "MQTT_PASSWORD":
			config.MQTTPassword = value
		}
	}

	return scanner.Err()
}
