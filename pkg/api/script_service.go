package api

import (
	"net/http"

	"mq_adb/pkg/script"

	"github.com/gin-gonic/gin"
)

// ScriptService 脚本服务
type ScriptService struct {
	engine *script.ScriptEngine
}

// NewScriptService 创建脚本服务
func NewScriptService(executor script.CommandExecutor) *ScriptService {
	return &ScriptService{
		engine: script.NewScriptEngine(executor),
	}
}

// ExecuteScript 执行脚本
func (s *ScriptService) ExecuteScript(c *gin.Context) {
	var request struct {
		DeviceID   string `json:"device_id" binding:"required"`
		ScriptName string `json:"script_name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	var result *script.ScriptResult
	var err error

	// 根据脚本名称执行对应脚本
	switch request.ScriptName {
	case "test_basic_commands":
		result = s.engine.TestBasicCommands(request.DeviceID)
	case "test_network_info":
		result = s.engine.TestNetworkInfo(request.DeviceID)
	case "custom_script":
		// 可以添加参数支持
		commands := []string{"echo Custom script", "ls -la", "echo Done"}
		result = s.engine.CustomScript(request.DeviceID, commands)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unknown script name",
			"available_scripts": []string{
				"test_basic_commands",
				"test_network_info",
				"custom_script",
			},
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to execute script",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result":  result,
	})
}

// ListScripts 列出可用脚本
func (s *ScriptService) ListScripts(c *gin.Context) {
	scripts := []map[string]interface{}{
		{
			"name":        "test_basic_commands",
			"description": "测试基本命令（echo, ls, adb命令）",
			"parameters":  []string{},
		},
		{
			"name":        "test_network_info",
			"description": "获取网络信息和设备状态",
			"parameters":  []string{},
		},
		{
			"name":        "custom_script",
			"description": "执行自定义命令序列",
			"parameters":  []string{"commands"},
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"scripts": scripts,
		"total":   len(scripts),
	})
}
