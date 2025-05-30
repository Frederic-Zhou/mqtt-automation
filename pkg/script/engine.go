package script

import (
	"fmt"
	"log"
	"time"
)

// CommandExecutor 命令执行接口
type CommandExecutor interface {
	ExecuteCommand(deviceID, command string, timeout int) (CommandExecutionInterface, error)
	WaitForCompletion(id string, maxWait time.Duration) (CommandExecutionInterface, error)
}

// CommandExecutionInterface 命令执行接口
type CommandExecutionInterface interface {
	GetID() string
	GetDeviceID() string
	GetCommand() string
	GetStatus() string
	GetStartTime() time.Time
	GetEndTime() *time.Time
	GetResponse() CommandResponseInterface
	GetError() string
}

// CommandResponseInterface 命令响应接口
type CommandResponseInterface interface {
	GetOutput() string
	GetStatus() string
	GetDuration() int64
	GetError() string
}

// ScriptEngine 脚本执行引擎
type ScriptEngine struct {
	executor CommandExecutor
}

// NewScriptEngine 创建脚本引擎
func NewScriptEngine(executor CommandExecutor) *ScriptEngine {
	return &ScriptEngine{
		executor: executor,
	}
}

// ScriptResult 脚本执行结果
type ScriptResult struct {
	Success   bool         `json:"success"`
	Message   string       `json:"message"`
	Steps     []StepResult `json:"steps"`
	Duration  int64        `json:"duration"`
	Timestamp int64        `json:"timestamp"`
}

// StepResult 步骤执行结果
type StepResult struct {
	Step        string `json:"step"`
	Command     string `json:"command"`
	Status      string `json:"status"`
	Output      string `json:"output"`
	Error       string `json:"error"`
	Duration    int64  `json:"duration"`
	ExecutionID string `json:"execution_id"`
}

// executeCommand 执行单个命令并等待结果
func (s *ScriptEngine) executeCommand(deviceID, command string, timeout int, description string) StepResult {
	result := StepResult{
		Step:    description,
		Command: command,
	}

	// 执行命令
	execution, err := s.executor.ExecuteCommand(deviceID, command, timeout)
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		return result
	}

	result.ExecutionID = execution.GetID()

	// 等待命令完成
	finalExecution, err := s.executor.WaitForCompletion(execution.GetID(), time.Duration(timeout+5)*time.Second)
	if err != nil {
		result.Status = "timeout"
		result.Error = err.Error()
		result.Duration = time.Since(execution.GetStartTime()).Milliseconds()
		return result
	}

	// 设置结果
	result.Status = finalExecution.GetStatus()
	if finalExecution.GetResponse() != nil {
		response := finalExecution.GetResponse()
		result.Output = response.GetOutput()
		result.Duration = response.GetDuration()
		if response.GetError() != "" {
			result.Error = response.GetError()
		}
	}
	if finalExecution.GetError() != "" {
		result.Error = finalExecution.GetError()
	}

	return result
}

// TestBasicCommands 测试基本命令的样例脚本
func (s *ScriptEngine) TestBasicCommands(deviceID string) *ScriptResult {
	startTime := time.Now()
	result := &ScriptResult{
		Success:   true,
		Message:   "测试基本命令",
		Steps:     []StepResult{},
		Timestamp: time.Now().Unix(),
	}

	log.Printf("开始执行脚本: 测试基本命令 (设备: %s)", deviceID)

	// 步骤1: Echo测试
	step1 := s.executeCommand(deviceID, "echo Hello from script", 5, "Echo测试")
	result.Steps = append(result.Steps, step1)
	if step1.Status != "completed" {
		result.Success = false
		result.Message = "Echo测试失败"
	}

	// 步骤2: 列出文件
	step2 := s.executeCommand(deviceID, "ls -la /", 10, "列出根目录文件")
	result.Steps = append(result.Steps, step2)
	if step2.Status != "completed" {
		result.Success = false
		if result.Message == "测试基本命令" {
			result.Message = "列出文件失败"
		}
	}

	// 步骤3: 获取Android版本信息
	step3 := s.executeCommand(deviceID, "adb shell getprop ro.build.version.release", 10, "获取Android版本")
	result.Steps = append(result.Steps, step3)
	if step3.Status != "completed" {
		result.Success = false
		if result.Message == "测试基本命令" {
			result.Message = "获取Android版本失败"
		}
	}

	// 步骤4: 获取设备型号
	step4 := s.executeCommand(deviceID, "adb shell getprop ro.product.model", 10, "获取设备型号")
	result.Steps = append(result.Steps, step4)
	if step4.Status != "completed" {
		result.Success = false
		if result.Message == "测试基本命令" {
			result.Message = "获取设备型号失败"
		}
	}

	// 步骤5: 获取屏幕分辨率
	step5 := s.executeCommand(deviceID, "adb shell wm size", 10, "获取屏幕分辨率")
	result.Steps = append(result.Steps, step5)
	if step5.Status != "completed" {
		result.Success = false
		if result.Message == "测试基本命令" {
			result.Message = "获取屏幕分辨率失败"
		}
	}

	result.Duration = time.Since(startTime).Milliseconds()

	if result.Success {
		result.Message = "所有基本命令测试完成"
		log.Printf("脚本执行成功: %s (耗时: %dms)", result.Message, result.Duration)
	} else {
		log.Printf("脚本执行失败: %s (耗时: %dms)", result.Message, result.Duration)
	}

	return result
}

// TestNetworkInfo 测试网络信息获取的样例脚本
func (s *ScriptEngine) TestNetworkInfo(deviceID string) *ScriptResult {
	startTime := time.Now()
	result := &ScriptResult{
		Success:   true,
		Message:   "测试网络信息获取",
		Steps:     []StepResult{},
		Timestamp: time.Now().Unix(),
	}

	log.Printf("开始执行脚本: 测试网络信息 (设备: %s)", deviceID)

	// 步骤1: 获取WiFi信息
	step1 := s.executeCommand(deviceID, "adb shell dumpsys wifi | grep 'mWifiInfo'", 15, "获取WiFi信息")
	result.Steps = append(result.Steps, step1)
	if step1.Status != "completed" {
		result.Success = false
		result.Message = "获取WiFi信息失败"
	}

	// 步骤2: 获取IP地址
	step2 := s.executeCommand(deviceID, "adb shell ip addr show wlan0", 10, "获取IP地址")
	result.Steps = append(result.Steps, step2)
	if step2.Status != "completed" {
		result.Success = false
		if result.Message == "测试网络信息获取" {
			result.Message = "获取IP地址失败"
		}
	}

	// 步骤3: Ping测试
	step3 := s.executeCommand(deviceID, "ping -c 3 8.8.8.8", 15, "Ping连通性测试")
	result.Steps = append(result.Steps, step3)
	if step3.Status != "completed" {
		result.Success = false
		if result.Message == "测试网络信息获取" {
			result.Message = "Ping测试失败"
		}
	}

	result.Duration = time.Since(startTime).Milliseconds()

	if result.Success {
		result.Message = "所有网络信息测试完成"
		log.Printf("脚本执行成功: %s (耗时: %dms)", result.Message, result.Duration)
	} else {
		log.Printf("脚本执行失败: %s (耗时: %dms)", result.Message, result.Duration)
	}

	return result
}

// CustomScript 自定义脚本执行
func (s *ScriptEngine) CustomScript(deviceID string, commands []string) *ScriptResult {
	startTime := time.Now()
	result := &ScriptResult{
		Success:   true,
		Message:   "自定义脚本执行",
		Steps:     []StepResult{},
		Timestamp: time.Now().Unix(),
	}

	log.Printf("开始执行自定义脚本 (设备: %s)", deviceID)

	for i, command := range commands {
		stepDesc := fmt.Sprintf("步骤%d: %s", i+1, command)
		step := s.executeCommand(deviceID, command, 10, stepDesc)
		result.Steps = append(result.Steps, step)

		if step.Status != "completed" {
			result.Success = false
			result.Message = fmt.Sprintf("步骤%d失败: %s", i+1, step.Error)
			break
		}
	}

	result.Duration = time.Since(startTime).Milliseconds()

	if result.Success {
		result.Message = "自定义脚本执行完成"
		log.Printf("脚本执行成功: %s (耗时: %dms)", result.Message, result.Duration)
	} else {
		log.Printf("脚本执行失败: %s (耗时: %dms)", result.Message, result.Duration)
	}

	return result
}
