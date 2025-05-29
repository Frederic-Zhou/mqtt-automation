package engine

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"mq_adb/pkg/models"
	"mq_adb/pkg/mqtt"

	"gopkg.in/yaml.v3"
)

// ScriptEngine 脚本执行引擎
type ScriptEngine struct {
	mqttClient    *mqtt.Client
	executions    map[string]*models.ExecutionContext
	mu            sync.RWMutex
	responseChans map[string]chan models.Response
}

// NewScriptEngine 创建新的脚本引擎
func NewScriptEngine(mqttClient *mqtt.Client) *ScriptEngine {
	return &ScriptEngine{
		mqttClient:    mqttClient,
		executions:    make(map[string]*models.ExecutionContext),
		responseChans: make(map[string]chan models.Response),
	}
}

// LoadScript 从YAML文件加载脚本
func (se *ScriptEngine) LoadScript(scriptName string) (*models.Script, error) {
	// 从文件系统加载脚本
	scriptPath := fmt.Sprintf("./scripts/%s.yaml", scriptName)
	log.Printf("Loading script '%s' from path: %s", scriptName, scriptPath)

	// 检查文件是否存在，如果不存在则从examples.yaml加载
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		log.Printf("Script file %s not found, falling back to examples.yaml", scriptPath)
		scriptPath = "./scripts/examples.yaml"
	}

	data, err := ioutil.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script file: %v", err)
	}

	log.Printf("Read script file, data length: %d", len(data))

	// 首先尝试解析单个脚本
	var singleScript models.Script
	if err := yaml.Unmarshal(data, &singleScript); err == nil && singleScript.Name != "" {
		log.Printf("Parsed single script: %s", singleScript.Name)
		if singleScript.Name == scriptName || scriptPath != "./scripts/examples.yaml" {
			return &singleScript, nil
		}
	} else {
		log.Printf("Failed to parse as single script: %v", err)
	}

	// 解析YAML文件，支持多个脚本
	var scripts []models.Script
	decoder := yaml.NewDecoder(bytes.NewReader(data))

	for {
		var script models.Script
		if err := decoder.Decode(&script); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Failed to decode script: %v", err)
			return nil, fmt.Errorf("failed to parse YAML: %v", err)
		}

		if script.Name != "" {
			log.Printf("Found script in multi-script file: %s", script.Name)
			scripts = append(scripts, script)
		}
	}

	// 查找指定名称的脚本
	for _, script := range scripts {
		if script.Name == scriptName {
			return &script, nil
		}
	}

	// 如果没找到，返回第一个脚本作为默认
	if len(scripts) > 0 {
		log.Printf("Script '%s' not found, using default script '%s'", scriptName, scripts[0].Name)
		return &scripts[0], nil
	}

	log.Printf("No scripts found in file %s", scriptPath)
	return nil, fmt.Errorf("no scripts found in file")
}

// ExecuteScript 执行脚本
func (se *ScriptEngine) ExecuteScript(request *models.ScriptRequest) (*models.ScriptResponse, error) {
	script, err := se.LoadScript(request.ScriptName)
	if err != nil {
		return nil, fmt.Errorf("load script failed: %v", err)
	}

	executionID := fmt.Sprintf("%s_%s_%d", request.DeviceID, request.ScriptName, time.Now().Unix())

	context := &models.ExecutionContext{
		ScriptName:  request.ScriptName,
		DeviceID:    request.DeviceID,
		Variables:   request.Variables,
		CurrentStep: 0,
		StartTime:   time.Now(),
		Status:      "running",
		Results:     make([]models.Response, 0),
	}

	se.mu.Lock()
	se.executions[executionID] = context
	se.responseChans[executionID] = make(chan models.Response, 10)
	se.mu.Unlock()

	// 启动异步执行
	go se.executeSteps(executionID, script, context)

	return &models.ScriptResponse{
		ExecutionID: executionID,
		Status:      "running",
		Message:     "Script execution started",
		StartTime:   context.StartTime,
	}, nil
}

// executeSteps 执行脚本步骤
func (se *ScriptEngine) executeSteps(executionID string, script *models.Script, context *models.ExecutionContext) {
	defer func() {
		se.mu.Lock()
		delete(se.responseChans, executionID)
		se.mu.Unlock()
	}()

	for i, step := range script.Steps {
		context.CurrentStep = i

		log.Printf("Executing step %d: %s", i, step.Name)

		// 替换变量
		command := se.replaceVariables(step, context.Variables)

		// 执行命令
		response, err := se.executeCommand(executionID, command, context.DeviceID)
		if err != nil {
			context.Status = "failed"
			log.Printf("Step %d failed: %v", i, err)
			break
		}

		context.Results = append(context.Results, *response)

		// 检查步骤结果
		if response.Status == "error" {
			if step.OnFailure != "" {
				// 跳转到指定步骤
				if step.OnFailure == "end" {
					context.Status = "failed"
					break
				}
				// 这里可以实现步骤跳转逻辑
			} else {
				context.Status = "failed"
				break
			}
		}

		// 等待时间
		if step.Wait > 0 {
			time.Sleep(time.Duration(step.Wait) * time.Second)
		}
	}

	if context.Status == "running" {
		context.Status = "completed"
	}

	log.Printf("Script execution %s completed with status: %s", executionID, context.Status)
}

// executeCommand 执行单个命令
func (se *ScriptEngine) executeCommand(executionID string, command *models.Command, deviceID string) (*models.Response, error) {
	command.ID = fmt.Sprintf("%s_%d", executionID, time.Now().UnixNano())
	command.DeviceID = deviceID
	command.Timestamp = time.Now().Unix()

	// 发送命令到设备
	topic := fmt.Sprintf("device/%s/command", deviceID)
	err := se.mqttClient.PublishCommand(topic, command)
	if err != nil {
		return nil, fmt.Errorf("publish command failed: %v", err)
	}

	// 等待响应
	responseChan := se.responseChans[executionID]
	timeout := time.Duration(30) * time.Second
	if command.Timeout > 0 {
		timeout = time.Duration(command.Timeout) * time.Second
	}

	select {
	case response := <-responseChan:
		if response.ID == command.ID {
			return &response, nil
		}
		// 如果收到的响应ID不匹配，继续等待
		return se.waitForResponse(responseChan, command.ID, timeout)
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

// waitForResponse 等待特定ID的响应
func (se *ScriptEngine) waitForResponse(responseChan chan models.Response, commandID string, timeout time.Duration) (*models.Response, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case response := <-responseChan:
			if response.ID == commandID {
				return &response, nil
			}
		case <-time.After(deadline.Sub(time.Now())):
			return &models.Response{
				ID:        commandID,
				Status:    "timeout",
				Error:     "response timeout",
				Timestamp: time.Now().Unix(),
			}, nil
		}
	}

	return nil, fmt.Errorf("timeout waiting for response")
}

// replaceVariables 替换命令中的变量
func (se *ScriptEngine) replaceVariables(step models.ScriptStep, variables map[string]interface{}) *models.Command {
	command := &models.Command{
		Type:    step.Type,
		Command: step.Command,
		X:       step.X,
		Y:       step.Y,
		Text:    step.Text,
		Timeout: step.Timeout,
	}

	// 替换文本中的变量
	if command.Text != "" {
		command.Text = se.substituteVariables(command.Text, variables)
	}

	if command.Command != "" {
		command.Command = se.substituteVariables(command.Command, variables)
	}

	return command
}

// substituteVariables 替换字符串中的变量
func (se *ScriptEngine) substituteVariables(text string, variables map[string]interface{}) string {
	result := text
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}
	return result
}

// HandleResponse 处理设备响应
func (se *ScriptEngine) HandleResponse(response *models.Response) {
	se.mu.RLock()
	defer se.mu.RUnlock()

	log.Printf("Received response for command ID: %s", response.ID)

	// 根据命令ID找到对应的执行上下文
	for executionID, responseChan := range se.responseChans {
		// 检查命令ID是否以执行ID开头（因为命令ID格式是 executionID_timestamp）
		if strings.HasPrefix(response.ID, executionID) {
			select {
			case responseChan <- *response:
				log.Printf("Response routed to execution %s", executionID)
				return
			default:
				log.Printf("Response channel full for execution %s", executionID)
			}
		}
	}

	log.Printf("No matching execution found for response ID: %s", response.ID)
}

// GetExecutionStatus 获取执行状态
func (se *ScriptEngine) GetExecutionStatus(executionID string) (*models.ExecutionContext, error) {
	se.mu.RLock()
	defer se.mu.RUnlock()

	context, exists := se.executions[executionID]
	if !exists {
		return nil, fmt.Errorf("execution not found")
	}

	return context, nil
}

// ListExecutions 列出所有执行
func (se *ScriptEngine) ListExecutions() map[string]*models.ExecutionContext {
	se.mu.RLock()
	defer se.mu.RUnlock()

	result := make(map[string]*models.ExecutionContext)
	for id, context := range se.executions {
		result[id] = context
	}
	return result
}
