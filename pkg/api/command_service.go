package api

import (
	"fmt"
	"sync"
	"time"

	"mq_adb/pkg/models"
	"mq_adb/pkg/scripts"
)

// CommandExecution 命令执行状态
type CommandExecution struct {
	ID        string           `json:"id"`
	DeviceID  string           `json:"device_id"`
	Command   string           `json:"command"`
	Status    string           `json:"status"` // pending, running, completed, failed, timeout
	StartTime time.Time        `json:"start_time"`
	EndTime   *time.Time       `json:"end_time,omitempty"`
	Request   *models.Command  `json:"request,omitempty"`
	Response  *models.Response `json:"response,omitempty"`
	Error     string           `json:"error,omitempty"`
}

// CommandService 命令执行服务
type CommandService struct {
	client     *scripts.MQTTClient
	executions map[string]*CommandExecution
	mutex      sync.RWMutex
}

// NewCommandService 创建命令服务
func NewCommandService() (*CommandService, error) {
	client, err := scripts.NewMQTTClient()
	if err != nil {
		return nil, fmt.Errorf("创建MQTT客户端失败: %v", err)
	}

	return &CommandService{
		client:     client,
		executions: make(map[string]*CommandExecution),
	}, nil
}

// ExecuteCommand 执行命令（编程接口）
func (s *CommandService) ExecuteCommand(deviceID, command string, timeout int) (*CommandExecution, error) {
	// 创建命令执行记录
	execution := &CommandExecution{
		ID:        fmt.Sprintf("%s_cmd_%d", deviceID, time.Now().Unix()),
		DeviceID:  deviceID,
		Command:   command,
		Status:    "pending",
		StartTime: time.Now(),
	}

	// 设置默认超时
	if timeout == 0 {
		timeout = 10 // 默认10秒
	}

	// 创建命令
	cmd := &models.Command{
		ID:        execution.ID,
		Command:   command,
		SerialNo:  deviceID,
		Timeout:   timeout,
		Timestamp: time.Now().Unix(),
	}
	execution.Request = cmd

	// 保存执行记录
	s.mutex.Lock()
	s.executions[execution.ID] = execution
	s.mutex.Unlock()

	// 异步执行命令
	go s.runCommand(execution)

	return execution, nil
}

// runCommand 运行命令
func (s *CommandService) runCommand(execution *CommandExecution) {
	// 更新状态为运行中
	s.updateExecutionStatus(execution.ID, "running", nil, "")

	// 执行命令
	response, err := s.client.ExecuteCommand(execution.Request)

	// 更新执行结果
	endTime := time.Now()
	execution.EndTime = &endTime
	execution.Response = response

	if err != nil {
		s.updateExecutionStatus(execution.ID, "failed", nil, err.Error())
	} else if response.Status == "success" {
		s.updateExecutionStatus(execution.ID, "completed", response, "")
	} else if response.Status == "timeout" {
		s.updateExecutionStatus(execution.ID, "timeout", response, response.Error)
	} else {
		s.updateExecutionStatus(execution.ID, "failed", response, response.Error)
	}
}

// updateExecutionStatus 更新执行状态
func (s *CommandService) updateExecutionStatus(id, status string, response *models.Response, error string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if execution, exists := s.executions[id]; exists {
		execution.Status = status
		if response != nil {
			execution.Response = response
		}
		if error != "" {
			execution.Error = error
		}
		if status == "completed" || status == "failed" || status == "timeout" {
			now := time.Now()
			execution.EndTime = &now
		}
	}
}

// GetExecution 获取执行状态
func (s *CommandService) GetExecution(id string) (*CommandExecution, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	execution, exists := s.executions[id]
	return execution, exists
}

// ListExecutions 列出所有执行记录
func (s *CommandService) ListExecutions() []*CommandExecution {
	s.mutex.RLock()
	executions := make([]*CommandExecution, 0, len(s.executions))
	for _, execution := range s.executions {
		executions = append(executions, execution)
	}
	s.mutex.RUnlock()

	// 按时间排序（最新的在前）
	for i := 0; i < len(executions); i++ {
		for j := i + 1; j < len(executions); j++ {
			if executions[i].StartTime.Before(executions[j].StartTime) {
				executions[i], executions[j] = executions[j], executions[i]
			}
		}
	}

	return executions
}

// CancelExecution 取消执行
func (s *CommandService) CancelExecution(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	execution, exists := s.executions[id]
	if !exists {
		return fmt.Errorf("执行记录不存在")
	}

	if execution.Status == "pending" {
		execution.Status = "cancelled"
		now := time.Now()
		execution.EndTime = &now
		execution.Error = "用户取消命令"
		return nil
	}

	return fmt.Errorf("命令已在执行中，无法取消")
}

// CleanupExecutions 清理旧的执行记录
func (s *CommandService) CleanupExecutions(maxAgeMinutes int) int {
	if maxAgeMinutes <= 0 {
		maxAgeMinutes = 60 // 默认清理1小时前的记录
	}

	cutoff := time.Now().Add(-time.Duration(maxAgeMinutes) * time.Minute)
	cleaned := 0

	s.mutex.Lock()
	for id, execution := range s.executions {
		if execution.StartTime.Before(cutoff) &&
			(execution.Status == "completed" || execution.Status == "failed" || execution.Status == "timeout" || execution.Status == "cancelled") {
			delete(s.executions, id)
			cleaned++
		}
	}
	s.mutex.Unlock()

	return cleaned
}

// GetStats 获取统计信息
func (s *CommandService) GetStats() map[string]interface{} {
	s.mutex.RLock()
	totalCommands := len(s.executions)
	runningCommands := 0
	for _, execution := range s.executions {
		if execution.Status == "running" || execution.Status == "pending" {
			runningCommands++
		}
	}
	s.mutex.RUnlock()

	return map[string]interface{}{
		"total_commands":   totalCommands,
		"running_commands": runningCommands,
		"timestamp":        time.Now(),
	}
}

// Stop 停止服务
func (s *CommandService) Stop() {
	if s.client != nil {
		s.client.Disconnect()
	}
}
