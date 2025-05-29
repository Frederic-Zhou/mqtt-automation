package engine

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
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

	data, err := os.ReadFile(scriptPath)
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
		ExecutionID: executionID,
		ScriptName:  request.ScriptName,
		DeviceID:    request.DeviceID,
		Variables:   request.Variables,
		RuntimeVars: make(map[string]interface{}),
		StepOutputs: make(map[string]map[string]interface{}),
		CurrentStep: 0,
		StartTime:   time.Now(),
		Status:      "running",
		Results:     make([]models.Response, 0),
	}

	// 将运行时变量复制到RuntimeVars中
	for k, v := range request.Variables {
		context.RuntimeVars[k] = v
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

		// 检查条件执行
		if step.Condition != "" && !se.evaluateCondition(step.Condition, context) {
			log.Printf("Step %d skipped due to condition: %s", i, step.Condition)
			continue
		}

		// 执行命令
		command := &models.Command{
			Type:    step.Type,
			Command: step.Command,
			Args:    step.Args,
			Text:    step.Text,
			Timeout: step.Timeout,
		}
		command.ExecutionID = executionID

		// 处理X和Y坐标（从interface{}转换为int）
		command.X = se.convertCoordinateToInt(step.X, context.RuntimeVars)
		command.Y = se.convertCoordinateToInt(step.Y, context.RuntimeVars)

		// 先进行基本的变量替换
		if command.Text != "" {
			command.Text = se.substituteVariables(command.Text, context.RuntimeVars)
		}
		if command.Command != "" {
			command.Command = se.substituteVariables(command.Command, context.RuntimeVars)
		}
		for j, arg := range command.Args {
			command.Args[j] = se.substituteVariables(arg, context.RuntimeVars)
		}

		response, err := se.executeCommand(executionID, command, context.DeviceID)
		if err != nil {
			context.Status = "failed"
			log.Printf("Step %d failed: %v", i, err)
			break
		}

		context.Results = append(context.Results, *response)

		// 处理步骤输出，更新RuntimeVars
		se.processStepOutput(step, response, context)

		// 如果这是tap命令且X或Y为0，尝试使用刚刚设置的变量
		if command.Type == "tap" && (command.X == 0 || command.Y == 0) {
			log.Printf("Post-processing tap command with dynamic coordinates")
			log.Printf("Available variables after step output: %+v", context.RuntimeVars)

			needReexecute := false

			if command.X == 0 {
				if xVar, exists := context.RuntimeVars["text_x"]; exists {
					if xInt, ok := se.convertToInt(xVar); ok {
						command.X = xInt
						needReexecute = true
						log.Printf("Updated X coordinate to: %d", xInt)
					}
				} else if xVar, exists := context.RuntimeVars["click_x"]; exists {
					if xInt, ok := se.convertToInt(xVar); ok {
						command.X = xInt
						needReexecute = true
						log.Printf("Updated X coordinate to: %d", xInt)
					}
				}
			}

			if command.Y == 0 {
				if yVar, exists := context.RuntimeVars["text_y"]; exists {
					if yInt, ok := se.convertToInt(yVar); ok {
						command.Y = yInt
						needReexecute = true
						log.Printf("Updated Y coordinate to: %d", yInt)
					}
				} else if yVar, exists := context.RuntimeVars["click_y"]; exists {
					if yInt, ok := se.convertToInt(yVar); ok {
						command.Y = yInt
						needReexecute = true
						log.Printf("Updated Y coordinate to: %d", yInt)
					}
				}
			}

			// 如果坐标被更新，重新执行tap命令
			if needReexecute && (command.X > 0 && command.Y > 0) {
				log.Printf("Re-executing tap command with coordinates: (%d, %d)", command.X, command.Y)

				retryResponse, retryErr := se.executeCommand(executionID, command, context.DeviceID)
				if retryErr != nil {
					log.Printf("Retry tap command failed: %v", retryErr)
				} else {
					// 替换原响应
					context.Results[len(context.Results)-1] = *retryResponse
					log.Printf("Tap command re-executed successfully")
				}
			}
		}

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
	command.ExecutionID = executionID
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
			ID:          command.ID,
			ExecutionID: executionID,
			Command:     command.Command,
			Status:      "timeout",
			Error:       "command execution timeout",
			Timestamp:   time.Now().Unix(),
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
		case <-time.After(time.Until(deadline)):
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

// processStepOutput 处理步骤输出数据
func (se *ScriptEngine) processStepOutput(step models.ScriptStep, response *models.Response, context *models.ExecutionContext) {
	log.Printf("processStepOutput called for step: %s", step.Name)
	log.Printf("Step OutputVars: %+v", step.OutputVars)

	if len(step.OutputVars) == 0 {
		log.Printf("No output vars defined for step: %s", step.Name)
		return
	}

	stepOutputs := make(map[string]interface{})

	// 处理不同类型的输出
	for varName, outputPath := range step.OutputVars {
		var value interface{}

		switch outputPath {
		case "result":
			value = response.Result
		case "status":
			value = response.Status
		case "error":
			value = response.Error
		case "screenshot":
			value = response.Screenshot
		default:
			// 处理复杂的输出路径，如 "text_info[0].x"
			log.Printf("Extracting value for path: %s", outputPath)
			value = se.extractValue(response, outputPath)
			log.Printf("Extracted value: %v", value)
		}

		stepOutputs[varName] = value
		context.RuntimeVars[varName] = value
		log.Printf("Step %s output: %s = %v", step.Name, varName, value)
	}

	if len(stepOutputs) > 0 {
		context.StepOutputs[step.Name] = stepOutputs
		log.Printf("RuntimeVars after processing: %+v", context.RuntimeVars)
	}
}

// extractValue 从响应中提取指定路径的值
func (se *ScriptEngine) extractValue(response *models.Response, path string) interface{} {
	// 简化版本的路径提取，支持基本的路径如 "text_info[0].x"
	parts := strings.Split(path, ".")

	if len(parts) == 0 {
		return nil
	}

	var current interface{} = response

	for _, part := range parts {
		// 处理数组索引，如 "text_info[0]"
		if strings.Contains(part, "[") && strings.Contains(part, "]") {
			// 提取字段名和索引
			fieldEnd := strings.Index(part, "[")
			field := part[:fieldEnd]
			indexPart := part[fieldEnd+1 : len(part)-1]

			// 获取字段值
			current = se.getFieldValue(current, field)
			if current == nil {
				return nil
			}

			// 处理数组索引
			if slice, ok := current.([]models.TextPosition); ok {
				if idx := se.parseIndex(indexPart); idx >= 0 && idx < len(slice) {
					current = slice[idx]
				} else {
					return nil
				}
			} else {
				return nil
			}
		} else {
			// 普通字段访问
			current = se.getFieldValue(current, part)
			if current == nil {
				return nil
			}
		}
	}

	return current
}

// getFieldValue 获取结构体字段值
func (se *ScriptEngine) getFieldValue(obj interface{}, field string) interface{} {
	switch v := obj.(type) {
	case *models.Response:
		switch field {
		case "result":
			return v.Result
		case "status":
			return v.Status
		case "error":
			return v.Error
		case "screenshot":
			return v.Screenshot
		case "text_info":
			return v.TextInfo
		case "output_data":
			return v.OutputData
		}
	case models.TextPosition:
		switch field {
		case "text":
			return v.Text
		case "x":
			return v.X
		case "y":
			return v.Y
		case "width":
			return v.Width
		case "height":
			return v.Height
		}
	}
	return nil
}

// parseIndex 解析索引字符串
func (se *ScriptEngine) parseIndex(indexStr string) int {
	if indexStr == "" {
		return -1
	}

	// 简单的数字解析
	if indexStr == "0" {
		return 0
	} else if indexStr == "1" {
		return 1
	} else if indexStr == "2" {
		return 2
	} else if indexStr == "3" {
		return 3
	} else if indexStr == "4" {
		return 4
	} else if indexStr == "5" {
		return 5
	}
	// 可以扩展支持更多索引或使用 strconv.Atoi
	return -1
}

// evaluateCondition 评估条件表达式
func (se *ScriptEngine) evaluateCondition(condition string, context *models.ExecutionContext) bool {
	// 简化版本的条件评估
	// 支持格式如: "var_name == 'value'" 或 "var_name != ''"

	if condition == "" {
		return true
	}

	// 处理简单的存在性检查，如 "found_text"
	if !strings.Contains(condition, "==") && !strings.Contains(condition, "!=") {
		value, exists := context.RuntimeVars[condition]
		return exists && value != nil && value != ""
	}

	// 处理比较表达式
	if strings.Contains(condition, "==") {
		parts := strings.Split(condition, "==")
		if len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.Trim(strings.TrimSpace(parts[1]), "'\"")

			if value, exists := context.RuntimeVars[left]; exists {
				return fmt.Sprintf("%v", value) == right
			}
		}
	} else if strings.Contains(condition, "!=") {
		parts := strings.Split(condition, "!=")
		if len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.Trim(strings.TrimSpace(parts[1]), "'\"")

			if value, exists := context.RuntimeVars[left]; exists {
				return fmt.Sprintf("%v", value) != right
			}
		}
	}

	return false
}

// convertToInt 将interface{}转换为int
func (se *ScriptEngine) convertToInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		// 尝试解析字符串为数字
		if v == "" {
			return 0, false
		}

		// 使用strconv.Atoi进行完整的字符串到数字转换
		if num, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return num, true
		}

		// 如果解析失败，记录日志
		log.Printf("Warning: Failed to convert string '%s' to int", v)
		return 0, false
	default:
		log.Printf("Warning: Cannot convert type %T to int", value)
		return 0, false
	}
}

// convertCoordinateToInt 将坐标值（可能是数字或变量字符串）转换为int
func (se *ScriptEngine) convertCoordinateToInt(value interface{}, variables map[string]interface{}) int {
	if value == nil {
		return 0
	}

	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		// 如果是空字符串，返回0
		if v == "" {
			return 0
		}

		// 如果是变量模板，先替换变量
		if strings.Contains(v, "{{") && strings.Contains(v, "}}") {
			substituted := se.substituteVariables(v, variables)
			// 递归调用处理替换后的值
			return se.convertCoordinateToInt(substituted, variables)
		}

		// 尝试直接解析为数字
		if num, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return num
		}

		log.Printf("Warning: Failed to convert coordinate string '%s' to int", v)
		return 0
	default:
		log.Printf("Warning: Cannot convert coordinate type %T to int", value)
		return 0
	}
}
