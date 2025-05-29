package models

import "time"

// Command 表示要执行的命令
type Command struct {
	ID        string            `json:"id"`                  // 命令唯一ID
	Type      string            `json:"type"`                // 命令类型: shell, tap, input, wait, check_text, screenshot
	Command   string            `json:"command,omitempty"`   // shell命令
	X         int               `json:"x,omitempty"`         // 点击坐标X
	Y         int               `json:"y,omitempty"`         // 点击坐标Y
	Text      string            `json:"text,omitempty"`      // 输入文本或查找文本
	Timeout   int               `json:"timeout,omitempty"`   // 超时时间(秒)
	Args      []string          `json:"args,omitempty"`      // 命令参数
	Variables map[string]string `json:"variables,omitempty"` // 变量替换
	DeviceID  string            `json:"device_id,omitempty"` // 设备ID
	Timestamp int64             `json:"timestamp,omitempty"`
}

// Response 表示命令执行结果
type Response struct {
	ID         string         `json:"id"`                   // 对应的命令ID
	Command    string         `json:"command"`              // 执行的命令
	Status     string         `json:"status"`               // success, error, timeout
	Result     string         `json:"result,omitempty"`     // 执行结果
	Error      string         `json:"error,omitempty"`      // 错误信息
	Screenshot string         `json:"screenshot,omitempty"` // base64编码的截图
	TextInfo   []TextPosition `json:"text_info,omitempty"`  // 屏幕文本位置信息
	Duration   int64          `json:"duration"`             // 执行耗时(毫秒)
	Timestamp  int64          `json:"timestamp"`
}

// TextPosition 表示屏幕上文本的位置
type TextPosition struct {
	Text   string `json:"text"`   // 文本内容
	X      int    `json:"x"`      // X坐标
	Y      int    `json:"y"`      // Y坐标
	Width  int    `json:"width"`  // 宽度
	Height int    `json:"height"` // 高度
}

// ScriptStep 表示脚本中的一个步骤
type ScriptStep struct {
	Name        string            `json:"name"`                  // 步骤名称
	Type        string            `json:"type"`                  // 命令类型
	Command     string            `json:"command,omitempty"`     // 命令内容
	X           int               `json:"x,omitempty"`           // 坐标X
	Y           int               `json:"y,omitempty"`           // 坐标Y
	Text        string            `json:"text,omitempty"`        // 文本内容
	Timeout     int               `json:"timeout,omitempty"`     // 超时时间
	Wait        int               `json:"wait,omitempty"`        // 等待时间
	Variables   map[string]string `json:"variables,omitempty"`   // 变量
	Conditions  []Condition       `json:"conditions,omitempty"`  // 条件判断
	OnSuccess   string            `json:"on_success,omitempty"`  // 成功后跳转步骤
	OnFailure   string            `json:"on_failure,omitempty"`  // 失败后跳转步骤
	RetryCount  int               `json:"retry_count,omitempty"` // 重试次数
	Description string            `json:"description,omitempty"` // 步骤描述
}

// Condition 表示条件判断
type Condition struct {
	Type     string `json:"type"`                // text_exists, text_not_exists, result_contains
	Value    string `json:"value"`               // 条件值
	Action   string `json:"action"`              // 满足条件时的动作
	NextStep string `json:"next_step,omitempty"` // 下一步骤
}

// Script 表示完整的脚本
type Script struct {
	Name        string                 `json:"name"`        // 脚本名称
	Description string                 `json:"description"` // 脚本描述
	Version     string                 `json:"version"`     // 版本号
	Variables   map[string]interface{} `json:"variables"`   // 全局变量
	Steps       []ScriptStep           `json:"steps"`       // 执行步骤
}

// ExecutionContext 表示脚本执行上下文
type ExecutionContext struct {
	ScriptName  string                 `json:"script_name"`
	DeviceID    string                 `json:"device_id"`
	Variables   map[string]interface{} `json:"variables"`
	CurrentStep int                    `json:"current_step"`
	StartTime   time.Time              `json:"start_time"`
	Status      string                 `json:"status"` // running, completed, failed, timeout
	Results     []Response             `json:"results"`
}

// ScriptRequest 表示执行脚本的请求
type ScriptRequest struct {
	DeviceID   string                 `json:"device_id"`   // 设备ID
	ScriptName string                 `json:"script_name"` // 脚本名称
	Variables  map[string]interface{} `json:"variables"`   // 输入变量
}

// ScriptResponse 表示脚本执行响应
type ScriptResponse struct {
	ExecutionID string     `json:"execution_id"` // 执行ID
	Status      string     `json:"status"`       // 执行状态
	Message     string     `json:"message"`      // 消息
	StartTime   time.Time  `json:"start_time"`   // 开始时间
	Duration    int64      `json:"duration"`     // 执行时长(毫秒)
	Results     []Response `json:"results"`      // 执行结果
}
