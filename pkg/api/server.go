package api

import (
	"net/http"
	"time"

	"mq_adb/pkg/engine"
	"mq_adb/pkg/models"

	"github.com/gin-gonic/gin"
)

// Server HTTP API服务器
type Server struct {
	engine *engine.ScriptEngine
	router *gin.Engine
}

// NewServer 创建新的API服务器
func NewServer(scriptEngine *engine.ScriptEngine) *Server {
	router := gin.Default()

	server := &Server{
		engine: scriptEngine,
		router: router,
	}

	server.setupRoutes()
	return server
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	api := s.router.Group("/api/v1")
	{
		// 执行脚本
		api.POST("/execute", s.executeScript)

		// 获取执行状态
		api.GET("/execution/:id", s.getExecutionStatus)

		// 列出所有执行
		api.GET("/executions", s.listExecutions)

		// 健康检查
		api.GET("/health", s.healthCheck)
	}

	// 静态文件服务（用于Web界面）
	s.router.Static("/static", "./web/static")
	s.router.LoadHTMLGlob("web/templates/*")

	// Web界面
	s.router.GET("/", s.webInterface)
	s.router.GET("/web", s.webInterface)
}

// executeScript 执行脚本
func (s *Server) executeScript(c *gin.Context) {
	var request models.ScriptRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// 验证必填字段
	if request.DeviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "device_id is required",
		})
		return
	}

	if request.ScriptName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "script_name is required",
		})
		return
	}

	// 执行脚本
	response, err := s.engine.ExecuteScript(&request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Script execution failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// getExecutionStatus 获取执行状态
func (s *Server) getExecutionStatus(c *gin.Context) {
	executionID := c.Param("id")

	context, err := s.engine.GetExecutionStatus(executionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Execution not found",
			"details": err.Error(),
		})
		return
	}

	// 计算持续时间
	duration := time.Since(context.StartTime).Milliseconds()
	if context.Status == "completed" || context.Status == "failed" {
		// 如果已完成，使用最后一个结果的时间戳
		if len(context.Results) > 0 {
			lastResult := context.Results[len(context.Results)-1]
			duration = lastResult.Timestamp*1000 - context.StartTime.UnixMilli()
		}
	}

	response := &models.ScriptResponse{
		ExecutionID: executionID,
		Status:      context.Status,
		StartTime:   context.StartTime,
		Duration:    duration,
		Results:     context.Results,
	}

	c.JSON(http.StatusOK, response)
}

// listExecutions 列出所有执行
func (s *Server) listExecutions(c *gin.Context) {
	executions := s.engine.ListExecutions()

	c.JSON(http.StatusOK, gin.H{
		"executions": executions,
	})
}

// healthCheck 健康检查
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	})
}

// webInterface Web界面
func (s *Server) webInterface(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "Mobile Automation Server",
	})
}

// Run 启动服务器
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}
