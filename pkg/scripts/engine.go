package scripts

import (
	"fmt"
	"log"
	"sync"
	"time"

	"mq_adb/pkg/models"
	"mq_adb/pkg/mqtt"
)

// GoScriptEngine Go脚本执行引擎
type GoScriptEngine struct {
	mqttClient    *mqtt.Client
	registry      *ScriptRegistry
	executions    map[string]*ScriptExecution
	mu            sync.RWMutex
	responseChans map[string]chan *models.Response
}

// ScriptExecution 脚本执行状态
type ScriptExecution struct {
	ID         string                 `json:"id"`
	ScriptName string                 `json:"script_name"`
	DeviceID   string                 `json:"device_id"`
	Variables  map[string]interface{} `json:"variables"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    *time.Time             `json:"end_time,omitempty"`
	Status     string                 `json:"status"` // running, completed, failed, cancelled
	Result     *ScriptResult          `json:"result,omitempty"`
	Context    *ScriptContext         `json:"-"`
}

// NewGoScriptEngine 创建新的Go脚本引擎
func NewGoScriptEngine(mqttClient *mqtt.Client) *GoScriptEngine {
	return &GoScriptEngine{
		mqttClient:    mqttClient,
		registry:      GlobalRegistry,
		executions:    make(map[string]*ScriptExecution),
		responseChans: make(map[string]chan *models.Response),
	}
}

// ExecuteScript 执行脚本
func (gse *GoScriptEngine) ExecuteScript(request *models.ScriptRequest) (*models.ScriptResponse, error) {
	// 生成执行ID
	executionID := fmt.Sprintf("%s_%s_%d", request.DeviceID, request.ScriptName, time.Now().Unix())

	// 检查脚本是否存在
	_, exists := gse.registry.Get(request.ScriptName)
	if !exists {
		return nil, fmt.Errorf("script '%s' not found", request.ScriptName)
	}

	// 创建脚本执行记录
	execution := &ScriptExecution{
		ID:         executionID,
		ScriptName: request.ScriptName,
		DeviceID:   request.DeviceID,
		Variables:  request.Variables,
		StartTime:  time.Now(),
		Status:     "running",
	}

	// 创建MQTT客户端
	client := NewMQTTScriptClient(gse.mqttClient, request.DeviceID)
	logger := &DefaultLogger{}

	// 创建脚本上下文
	context := NewScriptContext(request.DeviceID, executionID, request.Variables, client, logger)
	execution.Context = context

	// 存储执行记录
	gse.mu.Lock()
	gse.executions[executionID] = execution
	gse.responseChans[executionID] = make(chan *models.Response, 10)
	gse.mu.Unlock()

	// 启动异步执行
	go gse.executeScriptAsync(execution)

	return &models.ScriptResponse{
		ExecutionID: executionID,
		Status:      "running",
		Message:     "Script execution started",
		StartTime:   execution.StartTime,
	}, nil
}

// executeScriptAsync 异步执行脚本
func (gse *GoScriptEngine) executeScriptAsync(execution *ScriptExecution) {
	defer func() {
		// 清理响应通道
		gse.mu.Lock()
		delete(gse.responseChans, execution.ID)
		gse.mu.Unlock()

		// 处理panic
		if r := recover(); r != nil {
			log.Printf("Script execution panic: %v", r)
			execution.Status = "failed"
			endTime := time.Now()
			execution.EndTime = &endTime
			execution.Result = NewErrorResult(fmt.Sprintf("Script panic: %v", r), nil)
		}
	}()

	startTime := time.Now()

	// 执行脚本
	result, err := gse.registry.Execute(execution.ScriptName, execution.Context, execution.Variables)

	endTime := time.Now()
	execution.EndTime = &endTime

	if err != nil {
		execution.Status = "failed"
		execution.Result = NewErrorResult(fmt.Sprintf("Execution error: %v", err), err)
	} else {
		if result.Success {
			execution.Status = "completed"
		} else {
			execution.Status = "failed"
		}
		execution.Result = result
	}

	// 设置执行时长
	if execution.Result != nil {
		execution.Result.Duration = endTime.Sub(startTime)
	}

	log.Printf("Script execution %s completed with status: %s (duration: %v)",
		execution.ID, execution.Status, endTime.Sub(startTime))
}

// GetExecutionStatus 获取执行状态
func (gse *GoScriptEngine) GetExecutionStatus(executionID string) (*ScriptExecution, error) {
	gse.mu.RLock()
	defer gse.mu.RUnlock()

	execution, exists := gse.executions[executionID]
	if !exists {
		return nil, fmt.Errorf("execution not found")
	}

	return execution, nil
}

// ListExecutions 列出所有执行
func (gse *GoScriptEngine) ListExecutions() map[string]*ScriptExecution {
	gse.mu.RLock()
	defer gse.mu.RUnlock()

	result := make(map[string]*ScriptExecution)
	for id, execution := range gse.executions {
		result[id] = execution
	}
	return result
}

// CancelExecution 取消执行
func (gse *GoScriptEngine) CancelExecution(executionID string) error {
	gse.mu.RLock()
	execution, exists := gse.executions[executionID]
	gse.mu.RUnlock()

	if !exists {
		return fmt.Errorf("execution not found")
	}

	if execution.Status == "running" && execution.Context != nil {
		execution.Context.Cancel()
		execution.Status = "cancelled"
		endTime := time.Now()
		execution.EndTime = &endTime
		execution.Result = NewErrorResult("Execution cancelled by user", nil)
	}

	return nil
}

// ListAvailableScripts 列出可用的脚本
func (gse *GoScriptEngine) ListAvailableScripts() []string {
	return gse.registry.List()
}

// GetScriptInfo 获取脚本信息
func (gse *GoScriptEngine) GetScriptInfo() []ScriptInfo {
	return gse.registry.GetScriptInfo()
}

// HandleResponse 处理设备响应
func (gse *GoScriptEngine) HandleResponse(response *models.Response) {
	gse.mu.RLock()
	defer gse.mu.RUnlock()

	log.Printf("Received response: ID=%s, Status=%s", response.ID, response.Status)

	// 查找等待此响应的执行上下文
	for executionID, execution := range gse.executions {
		if execution.Context != nil && execution.Context.Client != nil {
			if mqttClient, ok := execution.Context.Client.(*MQTTScriptClient); ok {
				// 将响应传递给对应的客户端
				mqttClient.responseHandler.HandleResponse(response)
				log.Printf("Response forwarded to execution %s", executionID)
				return
			}
		}
	}

	log.Printf("No execution found for response ID: %s", response.ID)
}

// RegisterScript 注册自定义脚本
func (gse *GoScriptEngine) RegisterScript(name string, fn ScriptFunc) {
	gse.registry.Register(name, fn)
}

// CleanupOldExecutions 清理旧的执行记录
func (gse *GoScriptEngine) CleanupOldExecutions(maxAge time.Duration) int {
	gse.mu.Lock()
	defer gse.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	cleaned := 0

	for id, execution := range gse.executions {
		if execution.Status != "running" && execution.StartTime.Before(cutoff) {
			delete(gse.executions, id)
			cleaned++
		}
	}

	log.Printf("Cleaned up %d old executions", cleaned)
	return cleaned
}

// GetExecutionHistory 获取执行历史
func (gse *GoScriptEngine) GetExecutionHistory(limit int) []*ScriptExecution {
	gse.mu.RLock()
	defer gse.mu.RUnlock()

	var executions []*ScriptExecution
	for _, execution := range gse.executions {
		executions = append(executions, execution)
	}

	// 按开始时间排序（最新的在前面）
	for i := 0; i < len(executions)-1; i++ {
		for j := i + 1; j < len(executions); j++ {
			if executions[i].StartTime.Before(executions[j].StartTime) {
				executions[i], executions[j] = executions[j], executions[i]
			}
		}
	}

	// 限制返回数量
	if limit > 0 && len(executions) > limit {
		executions = executions[:limit]
	}

	return executions
}
