package scripts

import (
	"fmt"
	"sync"
)

// ScriptRegistry 脚本注册表
type ScriptRegistry struct {
	scripts map[string]ScriptFunc
	mu      sync.RWMutex
}

// ScriptInfo 脚本信息
type ScriptInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// NewScriptRegistry 创建新的脚本注册表
func NewScriptRegistry() *ScriptRegistry {
	return &ScriptRegistry{
		scripts: make(map[string]ScriptFunc),
	}
}

// Register 注册脚本函数
func (sr *ScriptRegistry) Register(name string, fn ScriptFunc) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.scripts[name] = fn
}

// Get 获取脚本函数
func (sr *ScriptRegistry) Get(name string) (ScriptFunc, bool) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	fn, exists := sr.scripts[name]
	return fn, exists
}

// List 列出所有已注册的脚本
func (sr *ScriptRegistry) List() []string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	names := make([]string, 0, len(sr.scripts))
	for name := range sr.scripts {
		names = append(names, name)
	}
	return names
}

// Execute 执行指定的脚本
func (sr *ScriptRegistry) Execute(name string, ctx *ScriptContext, params map[string]interface{}) (*ScriptResult, error) {
	fn, exists := sr.Get(name)
	if !exists {
		return nil, fmt.Errorf("script '%s' not found", name)
	}

	if params == nil {
		params = make(map[string]interface{})
	}

	ctx.Logger.Info("Executing script: %s", name)
	result := fn(ctx, params)

	if result.Success {
		ctx.Logger.Info("Script '%s' completed successfully: %s", name, result.Message)
	} else {
		ctx.Logger.Error("Script '%s' failed: %s", name, result.Message)
		if result.Error != "" {
			ctx.Logger.Error("Error details: %s", result.Error)
		}
	}

	return result, nil
}

// RegisterBuiltinScripts 注册所有内置脚本
func (sr *ScriptRegistry) RegisterBuiltinScripts() {
	// 注册内置脚本函数
	sr.Register("find_and_click", FindAndClickScript)
	sr.Register("login", LoginScript)
	sr.Register("screenshot", ScreenshotScript)
	sr.Register("smart_navigate", SmartNavigateScript)
	sr.Register("wait", WaitScript)
	sr.Register("input_text", InputTextScript)
	sr.Register("check_text", CheckTextScript)
	sr.Register("execute_shell", ExecuteShellScript)
	sr.Register("click_coordinate", ClickCoordinateScript)
}

// GetScriptInfo 获取脚本信息
func (sr *ScriptRegistry) GetScriptInfo() []ScriptInfo {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	infos := []ScriptInfo{
		{
			Name:        "find_and_click",
			Description: "查找文本并点击",
			Parameters: map[string]interface{}{
				"text":     "要查找的文本内容",
				"timeout":  "超时时间（秒），默认30",
				"required": "是否必须找到，默认true",
			},
		},
		{
			Name:        "login",
			Description: "自动登录功能",
			Parameters: map[string]interface{}{
				"username": "用户名",
				"password": "密码",
				"timeout":  "超时时间（秒），默认60",
			},
		},
		{
			Name:        "screenshot",
			Description: "截取屏幕截图",
			Parameters: map[string]interface{}{
				"save_path": "保存路径（可选）",
			},
		},
		{
			Name:        "smart_navigate",
			Description: "智能导航到指定应用",
			Parameters: map[string]interface{}{
				"app_name": "应用名称",
				"timeout":  "超时时间（秒），默认30",
			},
		},
		{
			Name:        "wait",
			Description: "等待指定时间",
			Parameters: map[string]interface{}{
				"seconds": "等待时间（秒）",
			},
		},
		{
			Name:        "input_text",
			Description: "输入文本",
			Parameters: map[string]interface{}{
				"text": "要输入的文本内容",
			},
		},
		{
			Name:        "check_text",
			Description: "检查文本是否存在",
			Parameters: map[string]interface{}{
				"text":     "要检查的文本内容",
				"required": "是否必须存在，默认true",
			},
		},
		{
			Name:        "execute_shell",
			Description: "执行Shell命令",
			Parameters: map[string]interface{}{
				"command": "要执行的Shell命令",
				"timeout": "超时时间（秒），默认30",
			},
		},
		{
			Name:        "click_coordinate",
			Description: "点击指定坐标",
			Parameters: map[string]interface{}{
				"x":       "X坐标（必需）",
				"y":       "Y坐标（必需）",
				"timeout": "超时时间（秒），默认30",
			},
		},
	}

	return infos
}

// 全局脚本注册表实例
var GlobalRegistry = NewScriptRegistry()

// init 初始化全局注册表
func init() {
	GlobalRegistry.RegisterBuiltinScripts()
}
