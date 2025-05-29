package models

import "time"

// Command 表示要执行的命令
type Command struct {
	ID          string            `json:"id"`                  // 命令唯一ID
	ExecutionID string            `json:"execution_id"`        // 脚本执行ID
	Type        string            `json:"type"`                // 命令类型: shell, tap, input, wait, check_text, screenshot
	Command     string            `json:"command,omitempty"`   // shell命令
	X           int               `json:"x,omitempty"`         // 点击坐标X
	Y           int               `json:"y,omitempty"`         // 点击坐标Y
	Text        string            `json:"text,omitempty"`      // 输入文本或查找文本
	Timeout     int               `json:"timeout,omitempty"`   // 超时时间(秒)
	Args        []string          `json:"args,omitempty"`      // 命令参数
	Variables   map[string]string `json:"variables,omitempty"` // 变量替换
	DeviceID    string            `json:"device_id,omitempty"` // 设备ID
	Timestamp   int64             `json:"timestamp,omitempty"`
}

// Response 表示命令执行结果
type Response struct {
	ID          string                 `json:"id"`                    // 对应的命令ID
	ExecutionID string                 `json:"execution_id"`          // 脚本执行ID
	Command     string                 `json:"command"`               // 执行的命令
	Status      string                 `json:"status"`                // success, error, timeout
	Result      string                 `json:"result,omitempty"`      // 执行结果
	Error       string                 `json:"error,omitempty"`       // 错误信息
	Screenshot  string                 `json:"screenshot,omitempty"`  // 截图文件路径或URL
	TextInfo    []TextPosition         `json:"text_info,omitempty"`   // 屏幕文本位置信息
	OutputData  map[string]interface{} `json:"output_data,omitempty"` // 步骤输出数据
	Duration    int64                  `json:"duration"`              // 执行耗时(毫秒)
	Timestamp   int64                  `json:"timestamp"`
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
	Name      string            `json:"name" yaml:"name"`                               // 步骤名称
	Type      string            `json:"type" yaml:"type"`                               // 命令类型
	Command   string            `json:"command,omitempty" yaml:"command,omitempty"`     // 命令内容
	Args      []string          `json:"args,omitempty" yaml:"args,omitempty"`           // 命令参数
	X         interface{}       `json:"x,omitempty" yaml:"x,omitempty"`                 // 坐标X (可以是int或string)
	Y         interface{}       `json:"y,omitempty" yaml:"y,omitempty"`                 // 坐标Y (可以是int或string)
	Text      string            `json:"text,omitempty" yaml:"text,omitempty"`           // 文本内容
	Timeout   int               `json:"timeout,omitempty" yaml:"timeout,omitempty"`     // 超时时间
	Wait      int               `json:"wait,omitempty" yaml:"wait,omitempty"`           // 等待时间
	Variables map[string]string `json:"variables,omitempty" yaml:"variables,omitempty"` // 变量

	// 新增：输出变量定义
	OutputVars map[string]string `json:"output_vars,omitempty" yaml:"output_vars,omitempty"` // 输出变量映射

	// 新增：条件执行
	Condition string `json:"condition,omitempty" yaml:"condition,omitempty"` // 执行条件

	Conditions  []Condition `json:"conditions,omitempty" yaml:"conditions,omitempty"`   // 条件判断
	OnSuccess   string      `json:"on_success,omitempty" yaml:"on_success,omitempty"`   // 成功后跳转步骤
	OnFailure   string      `json:"on_failure,omitempty" yaml:"on_failure,omitempty"`   // 失败后跳转步骤
	RetryCount  int         `json:"retry_count,omitempty" yaml:"retry_count,omitempty"` // 重试次数
	Description string      `json:"description,omitempty" yaml:"description,omitempty"` // 步骤描述
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
	ExecutionID string                 `json:"execution_id"` // 执行ID
	ScriptName  string                 `json:"script_name"`
	DeviceID    string                 `json:"device_id"`
	Variables   map[string]interface{} `json:"variables"`
	CurrentStep int                    `json:"current_step"`
	StartTime   time.Time              `json:"start_time"`
	Status      string                 `json:"status"` // running, completed, failed, timeout
	Results     []Response             `json:"results"`

	// 新增：运行时变量存储
	RuntimeVars map[string]interface{} `json:"runtime_vars"`

	// 新增：步骤输出数据
	StepOutputs map[string]map[string]interface{} `json:"step_outputs"`
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

// ScreenshotInfo 表示截图信息
type ScreenshotInfo struct {
	ExecutionID string    `json:"execution_id"` // 执行ID
	StepName    string    `json:"step_name"`    // 步骤名称
	DeviceID    string    `json:"device_id"`    // 设备ID
	Filename    string    `json:"filename"`     // 文件名
	LocalPath   string    `json:"local_path"`   // 本地路径
	ServerURL   string    `json:"server_url"`   // 服务器URL
	Timestamp   time.Time `json:"timestamp"`    // 时间戳
	FileSize    int64     `json:"file_size"`    // 文件大小
}
