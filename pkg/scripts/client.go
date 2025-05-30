package scripts

import (
	"fmt"
	"sync"
	"time"

	"mq_adb/pkg/models"
	"mq_adb/pkg/mqtt"
)

// MQTTScriptClient MQTT脚本客户端实现
type MQTTScriptClient struct {
	mqttClient      *mqtt.Client
	deviceID        string
	timeout         int // 默认超时时间（秒）
	responseHandler *ResponseWaiter
}

// ResponseWaiter 响应等待器
type ResponseWaiter struct {
	pendingCommands map[string]chan *models.Response
	mu              sync.RWMutex
}

// NewResponseWaiter 创建响应等待器
func NewResponseWaiter() *ResponseWaiter {
	return &ResponseWaiter{
		pendingCommands: make(map[string]chan *models.Response),
	}
}

// RegisterCommand 注册等待响应的命令
func (rw *ResponseWaiter) RegisterCommand(commandID string) chan *models.Response {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	ch := make(chan *models.Response, 1)
	rw.pendingCommands[commandID] = ch
	return ch
}

// HandleResponse 处理响应
func (rw *ResponseWaiter) HandleResponse(response *models.Response) {
	rw.mu.RLock()
	ch, exists := rw.pendingCommands[response.ID]
	rw.mu.RUnlock()

	if exists {
		select {
		case ch <- response:
			// 响应已发送
		default:
			// Channel已满，忽略
		}

		// 清理已完成的命令
		rw.mu.Lock()
		delete(rw.pendingCommands, response.ID)
		close(ch)
		rw.mu.Unlock()
	}
}

// NewMQTTScriptClient 创建新的MQTT脚本客户端
func NewMQTTScriptClient(mqttClient *mqtt.Client, deviceID string) *MQTTScriptClient {
	responseHandler := NewResponseWaiter()

	client := &MQTTScriptClient{
		mqttClient:      mqttClient,
		deviceID:        deviceID,
		timeout:         30, // 默认30秒超时
		responseHandler: responseHandler,
	}

	// 注意：不在这里设置响应处理器，由GoScriptEngine统一处理
	// mqttClient.SetResponseHandler(responseHandler.HandleResponse)

	return client
}

// SetTimeout 设置超时时间
func (msc *MQTTScriptClient) SetTimeout(seconds int) {
	msc.timeout = seconds
}

// ExecuteShell 执行Shell命令
func (msc *MQTTScriptClient) ExecuteShell(command string) (*models.Response, error) {
	cmd := &models.Command{
		ID:        msc.generateCommandID(),
		Type:      "shell",
		Command:   command,
		SerialNo:  msc.deviceID,
		Timeout:   msc.timeout,
		Timestamp: time.Now().Unix(),
	}

	return msc.executeCommand(cmd)
}

// Tap 点击坐标
func (msc *MQTTScriptClient) Tap(x, y int) (*models.Response, error) {
	cmd := &models.Command{
		ID:        msc.generateCommandID(),
		Type:      "tap",
		X:         x,
		Y:         y,
		SerialNo:  msc.deviceID,
		Timeout:   msc.timeout,
		Timestamp: time.Now().Unix(),
	}

	return msc.executeCommand(cmd)
}

// Input 输入文本
func (msc *MQTTScriptClient) Input(text string) (*models.Response, error) {
	cmd := &models.Command{
		ID:        msc.generateCommandID(),
		Type:      "input",
		Text:      text,
		SerialNo:  msc.deviceID,
		Timeout:   msc.timeout,
		Timestamp: time.Now().Unix(),
	}

	return msc.executeCommand(cmd)
}

// Screenshot 截图
func (msc *MQTTScriptClient) Screenshot() (*models.Response, error) {
	cmd := &models.Command{
		ID:        msc.generateCommandID(),
		Type:      "screenshot",
		SerialNo:  msc.deviceID,
		Timeout:   msc.timeout,
		Timestamp: time.Now().Unix(),
	}

	return msc.executeCommand(cmd)
}

// ScreenshotOnly 纯截图（不进行UI分析）
func (msc *MQTTScriptClient) ScreenshotOnly() (*models.Response, error) {
	cmd := &models.Command{
		ID:        msc.generateCommandID(),
		Type:      "screenshot_only",
		SerialNo:  msc.deviceID,
		Timeout:   msc.timeout,
		Timestamp: time.Now().Unix(),
	}

	return msc.executeCommand(cmd)
}

// GetUIText 获取UI文本信息
func (msc *MQTTScriptClient) GetUIText() (*models.Response, error) {
	cmd := &models.Command{
		ID:        msc.generateCommandID(),
		Type:      "get_ui_text",
		SerialNo:  msc.deviceID,
		Timeout:   msc.timeout,
		Timestamp: time.Now().Unix(),
	}

	return msc.executeCommand(cmd)
}

// GetOCRText 获取OCR文本信息
func (msc *MQTTScriptClient) GetOCRText(imageBase64 string) (*models.Response, error) {
	// 这个方法将在服务器端处理OCR，客户端只是占位符
	// 实际的OCR处理逻辑在服务器端实现
	return &models.Response{
		ID:        msc.generateCommandID(),
		Command:   "get_ocr_text",
		Status:    "error",
		Error:     "OCR processing should be handled on server side",
		Timestamp: time.Now().Unix(),
	}, fmt.Errorf("OCR processing should be handled on server side")
}

// CheckText 检查文本是否存在
func (msc *MQTTScriptClient) CheckText(text string) (*models.Response, error) {
	cmd := &models.Command{
		ID:        msc.generateCommandID(),
		Type:      "check_text",
		Text:      text,
		SerialNo:  msc.deviceID,
		Timeout:   msc.timeout,
		Timestamp: time.Now().Unix(),
	}

	return msc.executeCommand(cmd)
}

// Wait 等待指定时间
func (msc *MQTTScriptClient) Wait(seconds int) error {
	time.Sleep(time.Duration(seconds) * time.Second)
	return nil
}

// executeCommand 执行命令并等待响应
func (msc *MQTTScriptClient) executeCommand(command *models.Command) (*models.Response, error) {
	// 注册命令等待响应
	responseChan := msc.responseHandler.RegisterCommand(command.ID)

	// 发送命令到设备
	topic := fmt.Sprintf("device/no_%s/command", msc.deviceID)
	err := msc.mqttClient.PublishCommand(topic, command)
	if err != nil {
		return nil, fmt.Errorf("publish command failed: %v", err)
	}

	// 等待响应
	timeout := time.Duration(msc.timeout) * time.Second
	select {
	case response := <-responseChan:
		return response, nil
	case <-time.After(timeout):
		return &models.Response{
			ID:        command.ID,
			Command:   command.Command,
			Status:    "timeout",
			Error:     "command execution timeout",
			Timestamp: time.Now().Unix(),
		}, nil
	}
}

// generateCommandID 生成命令ID
func (msc *MQTTScriptClient) generateCommandID() string {
	return fmt.Sprintf("cmd_%s_%d", msc.deviceID, time.Now().UnixNano())
}

// MockScriptClient 模拟脚本客户端（用于测试）
type MockScriptClient struct {
	timeout int
	logger  ScriptLogger
}

// NewMockScriptClient 创建模拟客户端
func NewMockScriptClient(logger ScriptLogger) *MockScriptClient {
	return &MockScriptClient{
		timeout: 30,
		logger:  logger,
	}
}

func (msc *MockScriptClient) SetTimeout(seconds int) {
	msc.timeout = seconds
}

func (msc *MockScriptClient) ExecuteShell(command string) (*models.Response, error) {
	msc.logger.Info("Mock: Executing shell command: %s", command)
	return &models.Response{
		ID:        fmt.Sprintf("mock_%d", time.Now().UnixNano()),
		Command:   command,
		Status:    "success",
		Result:    "Mock shell execution result",
		Timestamp: time.Now().Unix(),
	}, nil
}

func (msc *MockScriptClient) Tap(x, y int) (*models.Response, error) {
	msc.logger.Info("Mock: Tapping at (%d, %d)", x, y)
	return &models.Response{
		ID:        fmt.Sprintf("mock_%d", time.Now().UnixNano()),
		Command:   fmt.Sprintf("tap %d %d", x, y),
		Status:    "success",
		Result:    fmt.Sprintf("Tapped at (%d, %d)", x, y),
		Timestamp: time.Now().Unix(),
	}, nil
}

func (msc *MockScriptClient) Input(text string) (*models.Response, error) {
	msc.logger.Info("Mock: Inputting text: %s", text)
	return &models.Response{
		ID:        fmt.Sprintf("mock_%d", time.Now().UnixNano()),
		Command:   "input",
		Status:    "success",
		Result:    fmt.Sprintf("Input text: %s", text),
		Timestamp: time.Now().Unix(),
	}, nil
}

func (msc *MockScriptClient) Screenshot() (*models.Response, error) {
	msc.logger.Info("Mock: Taking screenshot")

	// 模拟一些文本信息
	mockTextInfo := []models.TextPosition{
		{Text: "登录", X: 100, Y: 200, Width: 60, Height: 30, Source: "ui", Confidence: 100.0},
		{Text: "用户名", X: 50, Y: 150, Width: 80, Height: 25, Source: "ui", Confidence: 100.0},
		{Text: "密码", X: 50, Y: 180, Width: 60, Height: 25, Source: "ui", Confidence: 100.0},
		{Text: "确定", X: 200, Y: 250, Width: 50, Height: 30, Source: "ui", Confidence: 100.0},
	}

	return &models.Response{
		ID:         fmt.Sprintf("mock_%d", time.Now().UnixNano()),
		Command:    "screenshot",
		Status:     "success",
		Result:     "Screenshot taken",
		Screenshot: "mock_base64_screenshot_data",
		TextInfo:   mockTextInfo,
		Timestamp:  time.Now().Unix(),
	}, nil
}

func (msc *MockScriptClient) ScreenshotOnly() (*models.Response, error) {
	msc.logger.Info("Mock: Taking screenshot only (no UI analysis)")
	return &models.Response{
		ID:         fmt.Sprintf("mock_%d", time.Now().UnixNano()),
		Command:    "screenshot_only",
		Status:     "success",
		Result:     "Screenshot taken (no UI analysis)",
		Screenshot: "mock_base64_screenshot_data",
		Timestamp:  time.Now().Unix(),
	}, nil
}

func (msc *MockScriptClient) GetUIText() (*models.Response, error) {
	msc.logger.Info("Mock: Getting UI text information")

	// 模拟UI文本信息
	mockTextInfo := []models.TextPosition{
		{Text: "登录", X: 100, Y: 200, Width: 60, Height: 30, Source: "ui", Confidence: 100.0},
		{Text: "用户名", X: 50, Y: 150, Width: 80, Height: 25, Source: "ui", Confidence: 100.0},
		{Text: "密码", X: 50, Y: 180, Width: 60, Height: 25, Source: "ui", Confidence: 100.0},
		{Text: "确定", X: 200, Y: 250, Width: 50, Height: 30, Source: "ui", Confidence: 100.0},
	}

	return &models.Response{
		ID:        fmt.Sprintf("mock_%d", time.Now().UnixNano()),
		Command:   "get_ui_text",
		Status:    "success",
		Result:    "UI text extracted successfully",
		TextInfo:  mockTextInfo,
		Timestamp: time.Now().Unix(),
	}, nil
}

func (msc *MockScriptClient) GetOCRText(imageBase64 string) (*models.Response, error) {
	msc.logger.Info("Mock: Getting OCR text information")

	// 模拟OCR文本信息
	mockOCRTextInfo := []models.TextPosition{
		{Text: "Button", X: 120, Y: 300, Width: 80, Height: 35, Source: "ocr", Confidence: 85.5},
		{Text: "Settings", X: 80, Y: 400, Width: 100, Height: 30, Source: "ocr", Confidence: 92.3},
		{Text: "Cancel", X: 220, Y: 500, Width: 70, Height: 28, Source: "ocr", Confidence: 78.9},
	}

	return &models.Response{
		ID:        fmt.Sprintf("mock_%d", time.Now().UnixNano()),
		Command:   "get_ocr_text",
		Status:    "success",
		Result:    "OCR text extracted successfully",
		TextInfo:  mockOCRTextInfo,
		Timestamp: time.Now().Unix(),
	}, nil
}

func (msc *MockScriptClient) CheckText(text string) (*models.Response, error) {
	msc.logger.Info("Mock: Checking text: %s", text)

	// 模拟一些常见文本存在
	commonTexts := []string{"登录", "用户名", "密码", "确定", "取消", "设置"}
	found := false
	for _, commonText := range commonTexts {
		if text == commonText {
			found = true
			break
		}
	}

	status := "error"
	result := fmt.Sprintf("Text '%s' not found", text)
	if found {
		status = "success"
		result = fmt.Sprintf("Text '%s' found", text)
	}

	return &models.Response{
		ID:        fmt.Sprintf("mock_%d", time.Now().UnixNano()),
		Command:   "check_text",
		Status:    status,
		Result:    result,
		Timestamp: time.Now().Unix(),
	}, nil
}

func (msc *MockScriptClient) Wait(seconds int) error {
	msc.logger.Info("Mock: Waiting for %d seconds", seconds)
	// 在测试中不实际等待，只是模拟
	return nil
}
