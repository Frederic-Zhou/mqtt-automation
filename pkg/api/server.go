package api

import (
	"encoding/base64"
	"net/http"
	"time"

	"mq_adb/pkg/models"
	"mq_adb/pkg/ocr"
	"mq_adb/pkg/scripts"

	"github.com/gin-gonic/gin"
)

// GoScriptServer Go脚本API服务器
type GoScriptServer struct {
	engine *scripts.GoScriptEngine
	router *gin.Engine
}

// NewGoScriptServer 创建新的Go脚本API服务器
func NewGoScriptServer(scriptEngine *scripts.GoScriptEngine) *GoScriptServer {
	router := gin.Default()

	server := &GoScriptServer{
		engine: scriptEngine,
		router: router,
	}

	server.setupRoutes()
	return server
}

// setupRoutes 设置路由
func (s *GoScriptServer) setupRoutes() {
	api := s.router.Group("/api/v1")
	{
		// 脚本执行相关
		api.POST("/execute", s.executeScript)
		api.GET("/execution/:id", s.getExecutionStatus)
		api.DELETE("/execution/:id", s.cancelExecution)
		api.GET("/executions", s.listExecutions)
		api.GET("/executions/history", s.getExecutionHistory)

		// 脚本管理相关
		api.GET("/scripts", s.listScripts)
		api.GET("/scripts/info", s.getScriptInfo)

		// OCR 处理相关
		api.POST("/ocr/process", s.processOCR)
		api.POST("/ocr/process/:engine", s.processOCRWithEngine)
		api.GET("/ocr/engines", s.getOCREngines)
		api.GET("/ocr/engines/status", s.getOCREngineStatus)
		api.POST("/ocr/engines/default", s.setDefaultOCREngine)

		// 系统相关
		api.GET("/health", s.healthCheck)
		api.POST("/cleanup", s.cleanupExecutions)
	}

	// 静态文件服务（用于Web界面）
	s.router.Static("/static", "./web/static")
	s.router.LoadHTMLGlob("web/templates/*")

	// Web界面
	s.router.GET("/", s.webInterface)
	s.router.GET("/web", s.webInterface)
}

// executeScript 执行脚本
func (s *GoScriptServer) executeScript(c *gin.Context) {
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
func (s *GoScriptServer) getExecutionStatus(c *gin.Context) {
	executionID := c.Param("id")

	execution, err := s.engine.GetExecutionStatus(executionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Execution not found",
			"details": err.Error(),
		})
		return
	}

	// 构建响应
	response := map[string]interface{}{
		"id":          execution.ID,
		"script_name": execution.ScriptName,
		"device_id":   execution.DeviceID,
		"variables":   execution.Variables,
		"start_time":  execution.StartTime,
		"status":      execution.Status,
	}

	if execution.EndTime != nil {
		response["end_time"] = *execution.EndTime
		response["duration"] = execution.EndTime.Sub(execution.StartTime).Milliseconds()
	} else {
		response["duration"] = time.Since(execution.StartTime).Milliseconds()
	}

	if execution.Result != nil {
		response["result"] = execution.Result
	}

	c.JSON(http.StatusOK, response)
}

// cancelExecution 取消执行
func (s *GoScriptServer) cancelExecution(c *gin.Context) {
	executionID := c.Param("id")

	err := s.engine.CancelExecution(executionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Execution not found or cannot be cancelled",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Execution cancelled successfully",
	})
}

// listExecutions 列出所有执行
func (s *GoScriptServer) listExecutions(c *gin.Context) {
	executions := s.engine.ListExecutions()

	// 转换为更友好的格式
	result := make([]map[string]interface{}, 0, len(executions))
	for _, execution := range executions {
		item := map[string]interface{}{
			"id":          execution.ID,
			"script_name": execution.ScriptName,
			"device_id":   execution.DeviceID,
			"start_time":  execution.StartTime,
			"status":      execution.Status,
		}

		if execution.EndTime != nil {
			item["end_time"] = *execution.EndTime
			item["duration"] = execution.EndTime.Sub(execution.StartTime).Milliseconds()
		} else {
			item["duration"] = time.Since(execution.StartTime).Milliseconds()
		}

		if execution.Result != nil {
			item["success"] = execution.Result.Success
			item["message"] = execution.Result.Message
		}

		result = append(result, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"executions": result,
		"total":      len(result),
	})
}

// getExecutionHistory 获取执行历史
func (s *GoScriptServer) getExecutionHistory(c *gin.Context) {
	limit := 50 // 默认返回最近50条记录
	if l := c.Query("limit"); l != "" {
		if parsed, err := scripts.ConvertCoordinateToInt(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	executions := s.engine.GetExecutionHistory(limit)

	// 转换为API响应格式
	result := make([]map[string]interface{}, 0, len(executions))
	for _, execution := range executions {
		item := map[string]interface{}{
			"id":          execution.ID,
			"script_name": execution.ScriptName,
			"device_id":   execution.DeviceID,
			"start_time":  execution.StartTime,
			"status":      execution.Status,
		}

		if execution.EndTime != nil {
			item["end_time"] = *execution.EndTime
			item["duration"] = execution.EndTime.Sub(execution.StartTime).Milliseconds()
		}

		if execution.Result != nil {
			item["success"] = execution.Result.Success
			item["message"] = execution.Result.Message
			if execution.Result.Error != "" {
				item["error"] = execution.Result.Error
			}
		}

		result = append(result, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"history": result,
		"total":   len(result),
		"limit":   limit,
	})
}

// listScripts 列出可用脚本
func (s *GoScriptServer) listScripts(c *gin.Context) {
	scripts := s.engine.ListAvailableScripts()

	c.JSON(http.StatusOK, gin.H{
		"scripts": scripts,
		"total":   len(scripts),
	})
}

// getScriptInfo 获取脚本信息
func (s *GoScriptServer) getScriptInfo(c *gin.Context) {
	scriptInfo := s.engine.GetScriptInfo()

	c.JSON(http.StatusOK, gin.H{
		"scripts": scriptInfo,
		"total":   len(scriptInfo),
	})
}

// processOCR 处理 OCR 请求
func (s *GoScriptServer) processOCR(c *gin.Context) {
	var request struct {
		ImageBase64 string `json:"image_base64" binding:"required"`
		Languages   string `json:"languages,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// 解码 base64 图像
	imageData, err := base64.StdEncoding.DecodeString(request.ImageBase64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid base64 image data",
			"details": err.Error(),
		})
		return
	}

	// 处理 OCR
	languages := request.Languages
	if languages == "" {
		languages = "eng+chi_sim+jpn+kor" // 默认语言
	}

	textPositions, err := ocr.ProcessImage(imageData, languages)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "OCR processing failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"text_positions": textPositions,
		"total_found":    len(textPositions),
		"languages_used": languages,
	})
}

// processOCRWithEngine 使用指定引擎处理 OCR 请求
func (s *GoScriptServer) processOCRWithEngine(c *gin.Context) {
	engineType := c.Param("engine")

	var request struct {
		ImageBase64 string `json:"image_base64" binding:"required"`
		Languages   string `json:"languages,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// 解码 base64 图像
	imageData, err := base64.StdEncoding.DecodeString(request.ImageBase64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid base64 image data",
			"details": err.Error(),
		})
		return
	}

	// 处理 OCR
	languages := request.Languages
	if languages == "" {
		languages = "eng+chi_sim+jpn+kor" // 默认语言
	}

	textPositions, err := ocr.ProcessImageWithEngine(imageData, engineType, languages)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "OCR processing failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"engine_used":    engineType,
		"text_positions": textPositions,
		"total_found":    len(textPositions),
		"languages_used": languages,
	})
}

// getOCREngines 获取可用的 OCR 引擎列表
func (s *GoScriptServer) getOCREngines(c *gin.Context) {
	engines := ocr.GetAvailableEngines()

	c.JSON(http.StatusOK, gin.H{
		"engines": engines,
		"total":   len(engines),
	})
}

// getOCREngineStatus 获取 OCR 引擎状态
func (s *GoScriptServer) getOCREngineStatus(c *gin.Context) {
	status := ocr.GetEngineStatus()

	c.JSON(http.StatusOK, gin.H{
		"status": status,
	})
}

// setDefaultOCREngine 设置默认 OCR 引擎
func (s *GoScriptServer) setDefaultOCREngine(c *gin.Context) {
	var request struct {
		Engine string `json:"engine" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	err := ocr.SetDefaultEngine(request.Engine)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to set default engine",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Default OCR engine updated successfully",
		"default_engine": request.Engine,
	})
}

// cleanupExecutions 清理旧的执行记录
func (s *GoScriptServer) cleanupExecutions(c *gin.Context) {
	var request struct {
		MaxAgeHours int `json:"max_age_hours"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	if request.MaxAgeHours <= 0 {
		request.MaxAgeHours = 24 // 默认清理24小时前的记录
	}

	maxAge := time.Duration(request.MaxAgeHours) * time.Hour
	cleaned := s.engine.CleanupOldExecutions(maxAge)

	c.JSON(http.StatusOK, gin.H{
		"message": "Cleanup completed",
		"cleaned": cleaned,
		"max_age": request.MaxAgeHours,
	})
}

// healthCheck 健康检查
func (s *GoScriptServer) healthCheck(c *gin.Context) {
	executions := s.engine.ListExecutions()
	running := 0
	for _, execution := range executions {
		if execution.Status == "running" {
			running++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":             "ok",
		"timestamp":          time.Now().Unix(),
		"version":            "2.0.0-go-scripts",
		"script_engine":      "go",
		"total_executions":   len(executions),
		"running_executions": running,
		"available_scripts":  len(s.engine.ListAvailableScripts()),
	})
}

// webInterface Web界面
func (s *GoScriptServer) webInterface(c *gin.Context) {
	scripts := s.engine.ListAvailableScripts()
	scriptInfo := s.engine.GetScriptInfo()

	c.HTML(http.StatusOK, "index.html", gin.H{
		"title":       "Mobile Automation Server - Go Scripts",
		"scripts":     scripts,
		"script_info": scriptInfo,
		"version":     "2.0.0",
	})
}

// Run 启动服务器
func (s *GoScriptServer) Run(addr string) error {
	return s.router.Run(addr)
}
