package api

import (
	"net/http"
	"time"

	"mq_adb/pkg/script"

	"github.com/gin-gonic/gin"
)

// Server 服务端
type Server struct {
	router         *gin.Engine
	commandService *CommandService
	scriptService  *ScriptService
}

// 确保Server实现script.CommandExecutor接口
var _ script.CommandExecutor = (*Server)(nil)

// NewServer 创建服务端
func NewServer() (*Server, error) {
	commandService, err := NewCommandService()
	if err != nil {
		return nil, err
	}

	router := gin.Default()

	server := &Server{
		router:         router,
		commandService: commandService,
	}

	// 创建脚本服务（Server实现了CommandExecutor接口）
	server.scriptService = NewScriptService(server)

	server.setupRoutes()
	return server, nil
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	api := s.router.Group("/api/v1")
	{
		// 基础命令API
		api.POST("/command", s.executeCommand)
		api.GET("/command/:id", s.getCommandStatus)
		api.GET("/commands", s.listCommands)
		api.DELETE("/command/:id", s.cancelCommand)

		// 脚本API
		api.POST("/script/execute", s.scriptService.ExecuteScript)
		api.GET("/scripts", s.scriptService.ListScripts)

		// 系统API
		api.GET("/health", s.healthCheck)
		api.POST("/cleanup", s.cleanupCommands)
	}

	// 静态文件和Web界面
	s.router.Static("/static", "./web/static")
	s.router.LoadHTMLGlob("web/templates/*")
	s.router.GET("/", s.webInterface)
}

// executeCommand 执行命令（HTTP接口）
func (s *Server) executeCommand(c *gin.Context) {
	var request struct {
		DeviceID string `json:"device_id" binding:"required"`
		Command  string `json:"command" binding:"required"`
		Timeout  int    `json:"timeout,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// 调用命令服务
	execution, err := s.commandService.ExecuteCommand(request.DeviceID, request.Command, request.Timeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to execute command",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"execution_id": execution.ID,
		"message":      "Command submitted successfully",
	})
}

// getCommandStatus 获取命令状态
func (s *Server) getCommandStatus(c *gin.Context) {
	id := c.Param("id")

	execution, exists := s.commandService.GetExecution(id)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Command execution not found",
		})
		return
	}

	// 构建响应
	response := map[string]interface{}{
		"id":         execution.ID,
		"device_id":  execution.DeviceID,
		"command":    execution.Command,
		"status":     execution.Status,
		"start_time": execution.StartTime,
	}

	if execution.EndTime != nil {
		response["end_time"] = *execution.EndTime
		response["duration"] = execution.EndTime.Sub(execution.StartTime).Milliseconds()
	} else {
		response["duration"] = time.Since(execution.StartTime).Milliseconds()
	}

	if execution.Response != nil {
		response["output"] = execution.Response.Output
		response["response_status"] = execution.Response.Status
	}

	if execution.Error != "" {
		response["error"] = execution.Error
	}

	c.JSON(http.StatusOK, response)
}

// listCommands 列出所有命令
func (s *Server) listCommands(c *gin.Context) {
	executions := s.commandService.ListExecutions()

	// 构建响应
	result := make([]map[string]interface{}, 0, len(executions))
	for _, execution := range executions {
		item := map[string]interface{}{
			"id":         execution.ID,
			"device_id":  execution.DeviceID,
			"command":    execution.Command,
			"status":     execution.Status,
			"start_time": execution.StartTime,
		}

		if execution.EndTime != nil {
			item["end_time"] = *execution.EndTime
			item["duration"] = execution.EndTime.Sub(execution.StartTime).Milliseconds()
		} else {
			item["duration"] = time.Since(execution.StartTime).Milliseconds()
		}

		if execution.Response != nil {
			item["output"] = execution.Response.Output
		}

		if execution.Error != "" {
			item["error"] = execution.Error
		}

		result = append(result, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"commands": result,
		"total":    len(result),
	})
}

// cancelCommand 取消命令
func (s *Server) cancelCommand(c *gin.Context) {
	id := c.Param("id")

	err := s.commandService.CancelExecution(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Command cancelled successfully",
	})
}

// cleanupCommands 清理旧命令
func (s *Server) cleanupCommands(c *gin.Context) {
	var request struct {
		MaxAgeMinutes int `json:"max_age_minutes"`
	}

	if err := c.ShouldBindJSON(&request); err != nil || request.MaxAgeMinutes <= 0 {
		request.MaxAgeMinutes = 60 // 默认清理1小时前的记录
	}

	cleaned := s.commandService.CleanupExecutions(request.MaxAgeMinutes)

	c.JSON(http.StatusOK, gin.H{
		"message": "Cleanup completed",
		"cleaned": cleaned,
		"max_age": request.MaxAgeMinutes,
	})
}

// healthCheck 健康检查
func (s *Server) healthCheck(c *gin.Context) {
	stats := s.commandService.GetStats()
	stats["status"] = "healthy"

	c.JSON(http.StatusOK, stats)
}

// webInterface Web界面
func (s *Server) webInterface(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "移动设备自动化系统",
	})
}

// ExecuteCommand 实现script.CommandExecutor接口
func (s *Server) ExecuteCommand(deviceID, command string, timeout int) (script.CommandExecutionInterface, error) {
	execution, err := s.commandService.ExecuteCommand(deviceID, command, timeout)
	if err != nil {
		return nil, err
	}
	return execution, nil
}

// WaitForCompletion 实现script.CommandExecutor接口
func (s *Server) WaitForCompletion(id string, maxWait time.Duration) (script.CommandExecutionInterface, error) {
	return s.commandService.WaitForCompletion(id, maxWait)
}

// GetExecution 获取执行状态（原有的编程接口）
func (s *Server) GetExecution(id string) (*CommandExecution, bool) {
	return s.commandService.GetExecution(id)
}

// ExecuteCommandDirect 直接执行命令（原有的编程接口）
func (s *Server) ExecuteCommandDirect(deviceID, command string, timeout int) (*CommandExecution, error) {
	return s.commandService.ExecuteCommand(deviceID, command, timeout)
}

// WaitForCompletionDirect 等待命令完成（原有的编程接口）
func (s *Server) WaitForCompletionDirect(id string, maxWait time.Duration) (*CommandExecution, error) {
	return s.commandService.WaitForCompletion(id, maxWait)
}

// Run 启动服务器
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

// Stop 停止服务器
func (s *Server) Stop() {
	if s.commandService != nil {
		s.commandService.Stop()
	}
}
