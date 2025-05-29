package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

		// 截图管理
		api.POST("/screenshots/upload", s.uploadScreenshot)
		api.GET("/screenshots/:filename", s.getScreenshot)
		api.GET("/screenshots", s.listScreenshots)

		// 健康检查
		api.GET("/health", s.healthCheck)
	}

	// 静态文件服务（用于Web界面和截图）
	s.router.Static("/static", "./web/static")
	s.router.Static("/screenshots", "./screenshots")
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

// uploadScreenshot 上传截图
func (s *Server) uploadScreenshot(c *gin.Context) {
	file, err := c.FormFile("screenshot")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No file uploaded",
		})
		return
	}

	// 获取上传参数
	executionID := c.PostForm("execution_id")
	stepName := c.PostForm("step_name")
	deviceID := c.PostForm("device_id")

	if executionID == "" || deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "execution_id and device_id are required",
		})
		return
	}

	// 确保截图目录存在
	screenshotDir := "./screenshots"
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create screenshot directory",
		})
		return
	}

	// 生成文件名
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("screenshot_%s_%s.png", executionID, timestamp)
	localPath := filepath.Join(screenshotDir, filename)

	// 保存文件
	if err := c.SaveUploadedFile(file, localPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save file",
		})
		return
	}

	// 获取文件信息
	fileInfo, err := os.Stat(localPath)
	var fileSize int64 = 0
	if err == nil {
		fileSize = fileInfo.Size()
	}

	// 创建截图信息
	screenshotInfo := &models.ScreenshotInfo{
		ExecutionID: executionID,
		StepName:    stepName,
		DeviceID:    deviceID,
		Filename:    filename,
		LocalPath:   localPath,
		ServerURL:   fmt.Sprintf("/api/v1/screenshots/%s", filename),
		Timestamp:   time.Now(),
		FileSize:    fileSize,
	}

	c.JSON(http.StatusOK, screenshotInfo)
}

// getScreenshot 获取截图
func (s *Server) getScreenshot(c *gin.Context) {
	filename := c.Param("filename")

	// 验证文件名安全性
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid filename",
		})
		return
	}

	filepath := filepath.Join("./screenshots", filename)
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Screenshot not found",
		})
		return
	}

	c.File(filepath)
}

// listScreenshots 列出截图
func (s *Server) listScreenshots(c *gin.Context) {
	executionID := c.Query("execution_id")

	screenshotDir := "./screenshots"
	files, err := os.ReadDir(screenshotDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read screenshot directory",
		})
		return
	}

	var screenshots []models.ScreenshotInfo
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()
		if !strings.HasSuffix(filename, ".png") {
			continue
		}

		// 如果指定了executionID，则过滤
		if executionID != "" && !strings.Contains(filename, executionID) {
			continue
		}

		info, err := file.Info()
		var fileSize int64 = 0
		var timestamp time.Time = time.Now()
		if err == nil {
			fileSize = info.Size()
			timestamp = info.ModTime()
		}

		screenshots = append(screenshots, models.ScreenshotInfo{
			Filename:  filename,
			LocalPath: filepath.Join(screenshotDir, filename),
			ServerURL: fmt.Sprintf("/api/v1/screenshots/%s", filename),
			Timestamp: timestamp,
			FileSize:  fileSize,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"screenshots": screenshots,
		"count":       len(screenshots),
	})
}
