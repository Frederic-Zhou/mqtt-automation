package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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

	// 新增：文本查找缓存
	textCache    map[string][]models.TextPosition
	cacheMu      sync.RWMutex
	cacheTimeout time.Duration

	// 新增：持久化存储
	persistencePath string
}

// NewScriptEngine 创建新的脚本引擎
func NewScriptEngine(mqttClient *mqtt.Client) *ScriptEngine {
	persistencePath := "./data/executions"

	// 确保持久化目录存在
	if err := os.MkdirAll(persistencePath, 0755); err != nil {
		log.Printf("Warning: Failed to create persistence directory: %v", err)
	}

	engine := &ScriptEngine{
		mqttClient:      mqttClient,
		executions:      make(map[string]*models.ExecutionContext),
		responseChans:   make(map[string]chan models.Response),
		textCache:       make(map[string][]models.TextPosition),
		cacheTimeout:    10 * time.Second, // 默认缓存时间
		persistencePath: persistencePath,
	}

	// 加载历史执行记录
	engine.loadExecutionHistory()

	// 启动定期清理任务
	engine.startPeriodicCleanup()

	return engine
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

	// 将脚本全局变量复制到RuntimeVars中
	for k, v := range script.Variables {
		context.RuntimeVars[k] = v
	}

	// 将请求变量复制到RuntimeVars中（覆盖同名的脚本变量）
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

	// 创建步骤名称到索引的映射，用于步骤跳转
	stepMap := make(map[string]int)
	for i, step := range script.Steps {
		stepMap[step.Name] = i
	}

	i := 0
	for i < len(script.Steps) {
		step := script.Steps[i]
		context.CurrentStep = i

		log.Printf("Executing step %d: %s", i, step.Name)

		// 检查条件执行（使用增强的条件评估）
		if step.Condition != "" && !se.evaluateConditionExpression(step.Condition, context) {
			log.Printf("Step %d skipped due to condition: %s", i, step.Condition)
			i++ // 条件不满足时，直接跳过当前步骤
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

		// 检查步骤结果和处理跳转
		if response.Status == "error" {
			if step.OnFailure != "" {
				// 跳转到指定步骤
				targetStep := step.OnFailure
				log.Printf("Step %d failed, jumping to onFailure step: %s", i, targetStep)

				// 处理步骤跳转
				jumpIndex := se.handleStepJump(targetStep, stepMap, context)
				if jumpIndex >= 0 {
					i = jumpIndex // 跳转到指定步骤
					continue
				} else if jumpIndex == -2 {
					context.Status = "failed"
					break
				}
			} else {
				context.Status = "failed"
				break
			}
		} else if response.Status == "success" || response.Status == "ok" {
			if step.OnSuccess != "" {
				// 处理成功跳转
				targetStep := step.OnSuccess
				log.Printf("Step %d succeeded, jumping to onSuccess step: %s", i, targetStep)

				jumpIndex := se.handleStepJump(targetStep, stepMap, context)
				if jumpIndex >= 0 {
					i = jumpIndex // 跳转到指定步骤
					continue
				} else if jumpIndex == -2 {
					break // 正常结束
				}
				// 如果跳转失败（jumpIndex == -1），继续正常执行
			}
		}

		// 等待时间
		if step.Wait > 0 {
			time.Sleep(time.Duration(step.Wait) * time.Second)
		}

		i++ // 正常继续下一步
	}

	if context.Status == "running" {
		context.Status = "completed"
	}

	log.Printf("Script execution %s completed with status: %s", executionID, context.Status)

	// 保存执行记录到文件
	se.saveExecution(context)
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
		var valueStr string

		// 处理nil值，避免生成"<nil>"字符串
		if value == nil {
			valueStr = ""
		} else {
			valueStr = fmt.Sprintf("%v", value)
		}

		result = strings.ReplaceAll(result, placeholder, valueStr)
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
			// 处理复杂的输出路径，如 "text_info[0].x" 或 "text_info[text='设置'].x"
			log.Printf("Extracting value for path: %s", outputPath)

			// 先进行变量替换
			expandedPath := se.substituteVariables(outputPath, context.RuntimeVars)
			log.Printf("Expanded path: %s", expandedPath)

			value = se.extractValue(response, expandedPath)
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
	// 支持路径格式如 "text_info[0].x", "text_info[text='设置'].x"
	parts := strings.Split(path, ".")

	if len(parts) == 0 {
		return nil
	}

	var current interface{} = response

	for _, part := range parts {
		// 处理数组索引，如 "text_info[0]" 或 "text_info[text='设置']"
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
				var found bool = false

				// 支持多种查找语法
				if strings.HasPrefix(indexPart, "text='") && strings.HasSuffix(indexPart, "'") {
					// 精确文本匹配: text='设置'
					targetText := indexPart[6 : len(indexPart)-1]
					for i, textPos := range slice {
						if textPos.Text == targetText {
							current = slice[i]
							found = true
							centerX := textPos.X + textPos.Width/2
							centerY := textPos.Y + textPos.Height/2
							log.Printf("Found exact text '%s': original=(%d,%d), size=(%dx%d), center=(%d,%d)",
								targetText, textPos.X, textPos.Y, textPos.Width, textPos.Height, centerX, centerY)
							break
						}
					}
					if !found {
						log.Printf("Warning: Text '%s' not found in text_info array", targetText)
						return nil
					}
				} else if strings.HasPrefix(indexPart, "contains='") && strings.HasSuffix(indexPart, "'") {
					// 包含文本匹配: contains='设'
					targetText := indexPart[10 : len(indexPart)-1]
					for i, textPos := range slice {
						if strings.Contains(textPos.Text, targetText) {
							current = slice[i]
							found = true
							centerX := textPos.X + textPos.Width/2
							centerY := textPos.Y + textPos.Height/2
							log.Printf("Found text containing '%s': text='%s', original=(%d,%d), size=(%dx%d), center=(%d,%d)",
								targetText, textPos.Text, textPos.X, textPos.Y, textPos.Width, textPos.Height, centerX, centerY)
							break
						}
					}
					if !found {
						log.Printf("Warning: No text containing '%s' found in text_info array", targetText)
						return nil
					}
				} else if strings.HasPrefix(indexPart, "x>") {
					// X坐标条件查找: x>500
					threshold, err := strconv.Atoi(indexPart[2:])
					if err == nil {
						for i, textPos := range slice {
							if textPos.X > threshold {
								current = slice[i]
								found = true
								log.Printf("Found text with x>%d: '%s' at (%d, %d)", threshold, textPos.Text, textPos.X, textPos.Y)
								break
							}
						}
					}
					if !found {
						log.Printf("Warning: No text with x>%s found in text_info array", indexPart[2:])
						return nil
					}
				} else if strings.HasPrefix(indexPart, "y>") {
					// Y坐标条件查找: y>800
					threshold, err := strconv.Atoi(indexPart[2:])
					if err == nil {
						for i, textPos := range slice {
							if textPos.Y > threshold {
								current = slice[i]
								found = true
								log.Printf("Found text with y>%d: '%s' at (%d, %d)", threshold, textPos.Text, textPos.X, textPos.Y)
								break
							}
						}
					}
					if !found {
						log.Printf("Warning: No text with y>%s found in text_info array", indexPart[2:])
						return nil
					}
				} else {
					// 数字索引
					if idx := se.parseIndex(indexPart); idx >= 0 && idx < len(slice) {
						current = slice[idx]
						found = true
					}
				}
				if !found {
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
			// 返回文本区域的中心X坐标
			return v.X + v.Width/2
		case "y":
			// 返回文本区域的中心Y坐标
			return v.Y + v.Height/2
		case "center_x":
			// 显式获取中心X坐标的别名
			return v.X + v.Width/2
		case "center_y":
			// 显式获取中心Y坐标的别名
			return v.Y + v.Height/2
		case "left_x":
			// 获取左上角X坐标
			return v.X
		case "top_y":
			// 获取左上角Y坐标
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

	// 使用 strconv.Atoi 解析任意数字索引
	if idx, err := strconv.Atoi(indexStr); err == nil {
		return idx
	}

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

// handleStepJump 处理步骤跳转逻辑
func (se *ScriptEngine) handleStepJump(target string, stepMap map[string]int, context *models.ExecutionContext) int {
	switch target {
	case "end":
		log.Printf("Ending script execution due to step jump")
		return -2 // 特殊值表示结束执行
	case "next":
		return -1 // 继续下一步
	case "skip":
		return -1 // 跳过当前步骤，继续下一步
	default:
		// 尝试跳转到指定的步骤名称
		if stepIndex, exists := stepMap[target]; exists {
			log.Printf("Jumping to step '%s' at index %d", target, stepIndex)
			return stepIndex
		} else {
			log.Printf("Warning: Step '%s' not found for jump, continuing normally", target)
			return -1
		}
	}
}

// evaluateConditionExpression 评估复杂条件表达式
func (se *ScriptEngine) evaluateConditionExpression(condition string, context *models.ExecutionContext) bool {
	// 支持更复杂的条件表达式
	condition = strings.TrimSpace(condition)

	// 支持逻辑运算符 AND, OR
	if strings.Contains(condition, " AND ") {
		parts := strings.Split(condition, " AND ")
		for _, part := range parts {
			if !se.evaluateCondition(strings.TrimSpace(part), context) {
				return false
			}
		}
		return true
	}

	if strings.Contains(condition, " OR ") {
		parts := strings.Split(condition, " OR ")
		for _, part := range parts {
			if se.evaluateCondition(strings.TrimSpace(part), context) {
				return true
			}
		}
		return false
	}

	// 支持数值比较
	if strings.Contains(condition, ">=") {
		parts := strings.Split(condition, ">=")
		if len(parts) == 2 {
			left := se.getVariableValue(strings.TrimSpace(parts[0]), context)
			right := se.getVariableValue(strings.TrimSpace(parts[1]), context)
			return se.compareNumbers(left, right, ">=")
		}
	}

	if strings.Contains(condition, "<=") {
		parts := strings.Split(condition, "<=")
		if len(parts) == 2 {
			left := se.getVariableValue(strings.TrimSpace(parts[0]), context)
			right := se.getVariableValue(strings.TrimSpace(parts[1]), context)
			return se.compareNumbers(left, right, "<=")
		}
	}

	if strings.Contains(condition, ">") {
		parts := strings.Split(condition, ">")
		if len(parts) == 2 {
			left := se.getVariableValue(strings.TrimSpace(parts[0]), context)
			right := se.getVariableValue(strings.TrimSpace(parts[1]), context)
			return se.compareNumbers(left, right, ">")
		}
	}

	if strings.Contains(condition, "<") {
		parts := strings.Split(condition, "<")
		if len(parts) == 2 {
			left := se.getVariableValue(strings.TrimSpace(parts[0]), context)
			right := se.getVariableValue(strings.TrimSpace(parts[1]), context)
			return se.compareNumbers(left, right, "<")
		}
	}

	// 回退到原始条件评估
	return se.evaluateCondition(condition, context)
}

// getVariableValue 获取变量值或解析字面值
func (se *ScriptEngine) getVariableValue(expr string, context *models.ExecutionContext) interface{} {
	expr = strings.TrimSpace(expr)

	// 移除引号
	if (strings.HasPrefix(expr, "'") && strings.HasSuffix(expr, "'")) ||
		(strings.HasPrefix(expr, "\"") && strings.HasSuffix(expr, "\"")) {
		return expr[1 : len(expr)-1]
	}

	// 尝试解析为数字
	if num, err := strconv.ParseFloat(expr, 64); err == nil {
		return num
	}

	// 尝试从变量中获取
	if value, exists := context.RuntimeVars[expr]; exists {
		return value
	}

	// 如果是字符串字面值
	return expr
}

// compareNumbers 比较两个值（数字比较）
func (se *ScriptEngine) compareNumbers(left, right interface{}, operator string) bool {
	leftNum, leftOk := se.toNumber(left)
	rightNum, rightOk := se.toNumber(right)

	if !leftOk || !rightOk {
		log.Printf("Warning: Cannot compare non-numeric values: %v %s %v", left, operator, right)
		return false
	}

	switch operator {
	case ">":
		return leftNum > rightNum
	case "<":
		return leftNum < rightNum
	case ">=":
		return leftNum >= rightNum
	case "<=":
		return leftNum <= rightNum
	default:
		return false
	}
}

// toNumber 将接口值转换为数字
func (se *ScriptEngine) toNumber(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case string:
		if num, err := strconv.ParseFloat(v, 64); err == nil {
			return num, true
		}
	}
	return 0, false
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
		// 如果是空字符串或"<nil>"，返回0
		if v == "" || v == "<nil>" {
			return 0
		}

		// 如果是变量模板，先替换变量
		if strings.Contains(v, "{{") && strings.Contains(v, "}}") {
			substituted := se.substituteVariables(v, variables)

			// 防止无限递归：如果替换后的值和原值相同，直接返回0
			if substituted == v {
				log.Printf("Warning: Variable substitution resulted in unchanged value '%s', returning 0", v)
				return 0
			}

			// 防止<nil>递归：如果替换后是<nil>，直接返回0
			if substituted == "<nil>" {
				log.Printf("Warning: Variable substitution resulted in <nil>, returning 0")
				return 0
			}

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

// =============================================================================
// 持久化存储相关方法
// =============================================================================

// loadExecutionHistory 加载历史执行记录
func (se *ScriptEngine) loadExecutionHistory() {
	log.Printf("Loading execution history from: %s", se.persistencePath)

	files, err := os.ReadDir(se.persistencePath)
	if err != nil {
		log.Printf("Warning: Failed to read execution history directory: %v", err)
		return
	}

	loadedCount := 0
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(se.persistencePath, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("Warning: Failed to read execution file %s: %v", filePath, err)
			continue
		}

		var context models.ExecutionContext
		if err := json.Unmarshal(data, &context); err != nil {
			log.Printf("Warning: Failed to unmarshal execution file %s: %v", filePath, err)
			continue
		}

		// 将执行记录加载到内存中
		se.executions[context.ExecutionID] = &context
		loadedCount++
	}

	log.Printf("Loaded %d execution records from history", loadedCount)
}

// saveExecution 保存单个执行记录到文件
func (se *ScriptEngine) saveExecution(context *models.ExecutionContext) {
	if se.persistencePath == "" {
		return
	}

	fileName := fmt.Sprintf("%s.json", context.ExecutionID)
	filePath := filepath.Join(se.persistencePath, fileName)

	data, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		log.Printf("Warning: Failed to marshal execution context: %v", err)
		return
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Printf("Warning: Failed to save execution to file %s: %v", filePath, err)
	} else {
		log.Printf("Execution %s saved to %s", context.ExecutionID, filePath)
	}
}

// cleanupOldExecutions 清理过期的执行记录（保留最近30天的记录）
func (se *ScriptEngine) cleanupOldExecutions() {
	cutoffTime := time.Now().AddDate(0, 0, -30) // 30天前

	files, err := os.ReadDir(se.persistencePath)
	if err != nil {
		log.Printf("Warning: Failed to read execution directory for cleanup: %v", err)
		return
	}

	cleanedCount := 0
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(se.persistencePath, file.Name())
		info, err := file.Info()
		if err != nil {
			continue
		}

		// 如果文件修改时间早于截止时间，删除文件
		if info.ModTime().Before(cutoffTime) {
			if err := os.Remove(filePath); err != nil {
				log.Printf("Warning: Failed to remove old execution file %s: %v", filePath, err)
			} else {
				cleanedCount++
				log.Printf("Removed old execution file: %s", filePath)

				// 同时从内存中移除
				executionID := strings.TrimSuffix(file.Name(), ".json")
				se.mu.Lock()
				delete(se.executions, executionID)
				se.mu.Unlock()
			}
		}
	}

	if cleanedCount > 0 {
		log.Printf("Cleaned up %d old execution records", cleanedCount)
	}
}

// startPeriodicCleanup 启动定期清理任务
func (se *ScriptEngine) startPeriodicCleanup() {
	go func() {
		ticker := time.NewTicker(24 * time.Hour) // 每天清理一次
		defer ticker.Stop()

		for range ticker.C {
			se.cleanupOldExecutions()
		}
	}()
}
