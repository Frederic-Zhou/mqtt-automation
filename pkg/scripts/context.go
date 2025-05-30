package scripts

import (
	"context"
	"fmt"
	"log"
	"time"

	"mq_adb/pkg/models"
)

// ScriptContext 脚本执行上下文
type ScriptContext struct {
	// 基本信息
	DeviceID    string                 `json:"device_id"`
	ExecutionID string                 `json:"execution_id"`
	Variables   map[string]interface{} `json:"variables"`
	StartTime   time.Time              `json:"start_time"`

	// 客户端接口
	Client ScriptClient `json:"-"`
	Logger ScriptLogger `json:"-"`

	// 内部状态
	ctx    context.Context
	cancel context.CancelFunc
}

// ScriptResult 脚本执行结果
type ScriptResult struct {
	Success     bool                   `json:"success"`
	Message     string                 `json:"message"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Duration    time.Duration          `json:"duration"`
	Screenshot  string                 `json:"screenshot,omitempty"`
	TextInfo    []models.TextPosition  `json:"text_info,omitempty"`
	Coordinates *Coordinate            `json:"coordinates,omitempty"`
}

// Coordinate 坐标信息
type Coordinate struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// ScriptFunc 脚本函数类型
type ScriptFunc func(ctx *ScriptContext, params map[string]interface{}) *ScriptResult

// ScriptClient 设备客户端接口
type ScriptClient interface {
	// 执行Shell命令
	ExecuteShell(command string) (*models.Response, error)

	// 点击坐标
	Tap(x, y int) (*models.Response, error)

	// 输入文本
	Input(text string) (*models.Response, error)

	// 截图
	Screenshot() (*models.Response, error)

	// 检查文本是否存在
	CheckText(text string) (*models.Response, error)

	// 等待指定时间
	Wait(seconds int) error

	// 设置超时时间
	SetTimeout(seconds int)
}

// ScriptLogger 日志接口
type ScriptLogger interface {
	Info(format string, args ...interface{})
	Error(format string, args ...interface{})
	Debug(format string, args ...interface{})
	Warn(format string, args ...interface{})
}

// NewScriptContext 创建新的脚本上下文
func NewScriptContext(deviceID, executionID string, variables map[string]interface{}, client ScriptClient, logger ScriptLogger) *ScriptContext {
	ctx, cancel := context.WithCancel(context.Background())

	if variables == nil {
		variables = make(map[string]interface{})
	}

	return &ScriptContext{
		DeviceID:    deviceID,
		ExecutionID: executionID,
		Variables:   variables,
		StartTime:   time.Now(),
		Client:      client,
		Logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// GetVariable 获取变量值
func (sc *ScriptContext) GetVariable(key string) (interface{}, bool) {
	value, exists := sc.Variables[key]
	return value, exists
}

// SetVariable 设置变量值
func (sc *ScriptContext) SetVariable(key string, value interface{}) {
	sc.Variables[key] = value
}

// GetStringVariable 获取字符串变量
func (sc *ScriptContext) GetStringVariable(key string, defaultValue string) string {
	if value, exists := sc.Variables[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return defaultValue
}

// GetIntVariable 获取整数变量
func (sc *ScriptContext) GetIntVariable(key string, defaultValue int) int {
	if value, exists := sc.Variables[key]; exists {
		switch v := value.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			// 尝试解析字符串为整数
			if i, err := ConvertCoordinateToInt(v); err == nil {
				return i
			}
		}
	}
	return defaultValue
}

// IsCancelled 检查上下文是否已取消
func (sc *ScriptContext) IsCancelled() bool {
	select {
	case <-sc.ctx.Done():
		return true
	default:
		return false
	}
}

// Cancel 取消脚本执行
func (sc *ScriptContext) Cancel() {
	if sc.cancel != nil {
		sc.cancel()
	}
}

// Context 获取Go context
func (sc *ScriptContext) Context() context.Context {
	return sc.ctx
}

// NewSuccessResult 创建成功结果
func NewSuccessResult(message string, data map[string]interface{}) *ScriptResult {
	return &ScriptResult{
		Success: true,
		Message: message,
		Data:    data,
	}
}

// NewErrorResult 创建错误结果
func NewErrorResult(message string, err error) *ScriptResult {
	result := &ScriptResult{
		Success: false,
		Message: message,
	}

	if err != nil {
		result.Error = err.Error()
	}

	return result
}

// WithScreenshot 添加截图到结果
func (sr *ScriptResult) WithScreenshot(screenshot string) *ScriptResult {
	sr.Screenshot = screenshot
	return sr
}

// WithTextInfo 添加文本信息到结果
func (sr *ScriptResult) WithTextInfo(textInfo []models.TextPosition) *ScriptResult {
	sr.TextInfo = textInfo
	return sr
}

// WithCoordinates 添加坐标信息到结果
func (sr *ScriptResult) WithCoordinates(x, y int) *ScriptResult {
	sr.Coordinates = &Coordinate{X: x, Y: y}
	return sr
}

// WithDuration 设置执行时长
func (sr *ScriptResult) WithDuration(duration time.Duration) *ScriptResult {
	sr.Duration = duration
	return sr
}

// DefaultLogger 默认日志实现
type DefaultLogger struct{}

func (dl *DefaultLogger) Info(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}

func (dl *DefaultLogger) Error(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}

func (dl *DefaultLogger) Debug(format string, args ...interface{}) {
	log.Printf("[DEBUG] "+format, args...)
}

func (dl *DefaultLogger) Warn(format string, args ...interface{}) {
	log.Printf("[WARN] "+format, args...)
}

// ConvertCoordinateToInt 将坐标值转换为整数（复用现有函数逻辑）
func ConvertCoordinateToInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		// 处理字符串形式的数字
		var result int
		_, err := fmt.Sscanf(v, "%d", &result)
		return result, err
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}
