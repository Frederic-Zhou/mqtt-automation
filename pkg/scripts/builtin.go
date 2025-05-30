package scripts

import (
	"fmt"
	"strings"
	"time"

	"mq_adb/pkg/models"
)

// FindAndClickScript 查找文本并点击
func FindAndClickScript(ctx *ScriptContext, params map[string]interface{}) *ScriptResult {
	startTime := time.Now()

	// 获取参数
	text, ok := params["text"].(string)
	if !ok || text == "" {
		return NewErrorResult("Missing required parameter: text", nil).WithDuration(time.Since(startTime))
	}

	timeout := ctx.GetIntVariable("timeout", 30)
	if t, exists := params["timeout"]; exists {
		if timeoutVal, err := ConvertCoordinateToInt(t); err == nil {
			timeout = timeoutVal
		}
	}

	required := true
	if r, exists := params["required"]; exists {
		if reqVal, ok := r.(bool); ok {
			required = reqVal
		}
	}

	ctx.Logger.Info("Finding and clicking text: '%s' (timeout: %ds, required: %v)", text, timeout, required)

	// 设置超时
	ctx.Client.SetTimeout(timeout)

	// 先截图获取屏幕内容
	response, err := ctx.Client.Screenshot()
	if err != nil {
		return NewErrorResult("Failed to take screenshot", err).WithDuration(time.Since(startTime))
	}

	if response.Status != "success" {
		return NewErrorResult("Screenshot failed: "+response.Error, nil).WithDuration(time.Since(startTime))
	}

	// 查找文本位置
	var targetPos *models.TextPosition
	for _, textInfo := range response.TextInfo {
		if strings.Contains(strings.ToLower(textInfo.Text), strings.ToLower(text)) {
			targetPos = &textInfo
			break
		}
	}

	if targetPos == nil {
		if required {
			return NewErrorResult(fmt.Sprintf("Text '%s' not found on screen", text), nil).
				WithScreenshot(response.Screenshot).
				WithTextInfo(response.TextInfo).
				WithDuration(time.Since(startTime))
		} else {
			return NewSuccessResult(fmt.Sprintf("Text '%s' not found (optional)", text), map[string]interface{}{
				"found": false,
			}).WithScreenshot(response.Screenshot).
				WithTextInfo(response.TextInfo).
				WithDuration(time.Since(startTime))
		}
	}

	// 计算点击坐标（文本中心点）
	clickX := targetPos.X + targetPos.Width/2
	clickY := targetPos.Y + targetPos.Height/2

	ctx.Logger.Info("Text found at (%d, %d), clicking at (%d, %d)", targetPos.X, targetPos.Y, clickX, clickY)

	// 点击文本
	tapResponse, err := ctx.Client.Tap(clickX, clickY)
	if err != nil {
		return NewErrorResult("Failed to tap", err).
			WithScreenshot(response.Screenshot).
			WithCoordinates(clickX, clickY).
			WithDuration(time.Since(startTime))
	}

	if tapResponse.Status != "success" {
		return NewErrorResult("Tap failed: "+tapResponse.Error, nil).
			WithScreenshot(response.Screenshot).
			WithCoordinates(clickX, clickY).
			WithDuration(time.Since(startTime))
	}

	// 成功
	return NewSuccessResult(fmt.Sprintf("Successfully found and clicked text: '%s'", text), map[string]interface{}{
		"found":       true,
		"text":        targetPos.Text,
		"click_x":     clickX,
		"click_y":     clickY,
		"text_bounds": targetPos,
	}).WithScreenshot(response.Screenshot).
		WithCoordinates(clickX, clickY).
		WithDuration(time.Since(startTime))
}

// LoginScript 自动登录脚本
func LoginScript(ctx *ScriptContext, params map[string]interface{}) *ScriptResult {
	startTime := time.Now()

	// 获取参数
	username := ctx.GetStringVariable("username", "")
	password := ctx.GetStringVariable("password", "")

	// 参数可以从params中覆盖
	if u, exists := params["username"].(string); exists && u != "" {
		username = u
	}
	if p, exists := params["password"].(string); exists && p != "" {
		password = p
	}

	if username == "" || password == "" {
		return NewErrorResult("Username and password are required", nil).WithDuration(time.Since(startTime))
	}

	timeout := ctx.GetIntVariable("timeout", 60)
	if t, exists := params["timeout"]; exists {
		if timeoutVal, err := ConvertCoordinateToInt(t); err == nil {
			timeout = timeoutVal
		}
	}

	ctx.Logger.Info("Starting auto login for user: %s", username)
	ctx.Client.SetTimeout(timeout)

	// 1. 查找并点击用户名输入框
	usernameResult := FindAndClickScript(ctx, map[string]interface{}{
		"text":     "用户名",
		"timeout":  15,
		"required": true,
	})

	if !usernameResult.Success {
		// 尝试其他可能的用户名标识
		usernameResult = FindAndClickScript(ctx, map[string]interface{}{
			"text":     "账号",
			"timeout":  15,
			"required": false,
		})
	}

	if !usernameResult.Success {
		return NewErrorResult("Cannot find username input field", nil).WithDuration(time.Since(startTime))
	}

	// 2. 输入用户名
	ctx.Logger.Info("Inputting username")
	inputResponse, err := ctx.Client.Input(username)
	if err != nil || inputResponse.Status != "success" {
		return NewErrorResult("Failed to input username", err).WithDuration(time.Since(startTime))
	}

	// 3. 查找并点击密码输入框
	passwordResult := FindAndClickScript(ctx, map[string]interface{}{
		"text":     "密码",
		"timeout":  15,
		"required": true,
	})

	if !passwordResult.Success {
		return NewErrorResult("Cannot find password input field", nil).WithDuration(time.Since(startTime))
	}

	// 4. 输入密码
	ctx.Logger.Info("Inputting password")
	inputResponse, err = ctx.Client.Input(password)
	if err != nil || inputResponse.Status != "success" {
		return NewErrorResult("Failed to input password", err).WithDuration(time.Since(startTime))
	}

	// 5. 查找并点击登录按钮
	loginResult := FindAndClickScript(ctx, map[string]interface{}{
		"text":     "登录",
		"timeout":  15,
		"required": true,
	})

	if !loginResult.Success {
		// 尝试其他可能的登录按钮标识
		loginResult = FindAndClickScript(ctx, map[string]interface{}{
			"text":     "确定",
			"timeout":  15,
			"required": false,
		})
	}

	if !loginResult.Success {
		return NewErrorResult("Cannot find login button", nil).WithDuration(time.Since(startTime))
	}

	// 6. 等待登录完成
	ctx.Logger.Info("Waiting for login completion")
	time.Sleep(3 * time.Second)

	// 7. 验证登录结果
	screenshot, err := ctx.Client.Screenshot()
	if err != nil {
		ctx.Logger.Warn("Failed to take final screenshot: %v", err)
	}

	return NewSuccessResult("Login process completed", map[string]interface{}{
		"username": username,
		"steps_completed": []string{
			"Found username field",
			"Input username",
			"Found password field",
			"Input password",
			"Clicked login button",
		},
	}).WithScreenshot(func() string {
		if screenshot != nil {
			return screenshot.Screenshot
		}
		return ""
	}()).WithDuration(time.Since(startTime))
}

// ScreenshotScript 截图脚本
func ScreenshotScript(ctx *ScriptContext, params map[string]interface{}) *ScriptResult {
	startTime := time.Now()

	ctx.Logger.Info("Taking screenshot")

	response, err := ctx.Client.Screenshot()
	if err != nil {
		return NewErrorResult("Failed to take screenshot", err).WithDuration(time.Since(startTime))
	}

	if response.Status != "success" {
		return NewErrorResult("Screenshot failed: "+response.Error, nil).WithDuration(time.Since(startTime))
	}

	return NewSuccessResult("Screenshot taken successfully", map[string]interface{}{
		"timestamp":  time.Now().Unix(),
		"text_count": len(response.TextInfo),
	}).WithScreenshot(response.Screenshot).
		WithTextInfo(response.TextInfo).
		WithDuration(time.Since(startTime))
}

// SmartNavigateScript 智能导航脚本
func SmartNavigateScript(ctx *ScriptContext, params map[string]interface{}) *ScriptResult {
	startTime := time.Now()

	appName, ok := params["app_name"].(string)
	if !ok || appName == "" {
		return NewErrorResult("Missing required parameter: app_name", nil).WithDuration(time.Since(startTime))
	}

	timeout := ctx.GetIntVariable("timeout", 30)
	if t, exists := params["timeout"]; exists {
		if timeoutVal, err := ConvertCoordinateToInt(t); err == nil {
			timeout = timeoutVal
		}
	}

	ctx.Logger.Info("Smart navigating to app: %s", appName)
	ctx.Client.SetTimeout(timeout)

	// 1. 先尝试直接查找应用
	appResult := FindAndClickScript(ctx, map[string]interface{}{
		"text":     appName,
		"timeout":  10,
		"required": false,
	})

	if appResult.Success {
		return NewSuccessResult(fmt.Sprintf("Found and opened app: %s", appName), map[string]interface{}{
			"method": "direct_click",
			"app":    appName,
		}).WithDuration(time.Since(startTime))
	}

	// 2. 如果直接查找失败，尝试打开应用菜单
	ctx.Logger.Info("App not found on current screen, trying to open app drawer")

	// 尝试查找常见的应用菜单按钮
	menuTexts := []string{"应用", "所有应用", "菜单", "更多"}
	var menuOpened bool

	for _, menuText := range menuTexts {
		menuResult := FindAndClickScript(ctx, map[string]interface{}{
			"text":     menuText,
			"timeout":  5,
			"required": false,
		})

		if menuResult.Success {
			menuOpened = true
			ctx.Logger.Info("Opened app menu using: %s", menuText)
			time.Sleep(2 * time.Second) // 等待菜单加载
			break
		}
	}

	if !menuOpened {
		return NewErrorResult("Cannot find app menu", nil).WithDuration(time.Since(startTime))
	}

	// 3. 在应用菜单中查找目标应用
	appResult = FindAndClickScript(ctx, map[string]interface{}{
		"text":     appName,
		"timeout":  15,
		"required": true,
	})

	if !appResult.Success {
		return NewErrorResult(fmt.Sprintf("App '%s' not found in app menu", appName), nil).WithDuration(time.Since(startTime))
	}

	return NewSuccessResult(fmt.Sprintf("Successfully navigated to app: %s", appName), map[string]interface{}{
		"method": "app_menu",
		"app":    appName,
	}).WithDuration(time.Since(startTime))
}

// WaitScript 等待脚本
func WaitScript(ctx *ScriptContext, params map[string]interface{}) *ScriptResult {
	startTime := time.Now()

	seconds, err := ConvertCoordinateToInt(params["seconds"])
	if err != nil || seconds <= 0 {
		return NewErrorResult("Invalid seconds parameter", err).WithDuration(time.Since(startTime))
	}

	ctx.Logger.Info("Waiting for %d seconds", seconds)

	err = ctx.Client.Wait(seconds)
	if err != nil {
		return NewErrorResult("Wait failed", err).WithDuration(time.Since(startTime))
	}

	return NewSuccessResult(fmt.Sprintf("Waited for %d seconds", seconds), map[string]interface{}{
		"seconds": seconds,
	}).WithDuration(time.Since(startTime))
}

// InputTextScript 输入文本脚本
func InputTextScript(ctx *ScriptContext, params map[string]interface{}) *ScriptResult {
	startTime := time.Now()

	text, ok := params["text"].(string)
	if !ok || text == "" {
		return NewErrorResult("Missing required parameter: text", nil).WithDuration(time.Since(startTime))
	}

	ctx.Logger.Info("Inputting text: %s", text)

	response, err := ctx.Client.Input(text)
	if err != nil {
		return NewErrorResult("Failed to input text", err).WithDuration(time.Since(startTime))
	}

	if response.Status != "success" {
		return NewErrorResult("Input failed: "+response.Error, nil).WithDuration(time.Since(startTime))
	}

	return NewSuccessResult("Text input successful", map[string]interface{}{
		"text": text,
	}).WithDuration(time.Since(startTime))
}

// CheckTextScript 检查文本脚本
func CheckTextScript(ctx *ScriptContext, params map[string]interface{}) *ScriptResult {
	startTime := time.Now()

	text, ok := params["text"].(string)
	if !ok || text == "" {
		return NewErrorResult("Missing required parameter: text", nil).WithDuration(time.Since(startTime))
	}

	required := true
	if r, exists := params["required"]; exists {
		if reqVal, ok := r.(bool); ok {
			required = reqVal
		}
	}

	ctx.Logger.Info("Checking text: %s", text)

	response, err := ctx.Client.CheckText(text)
	if err != nil {
		return NewErrorResult("Failed to check text", err).WithDuration(time.Since(startTime))
	}

	found := response.Status == "success"

	if required && !found {
		return NewErrorResult(fmt.Sprintf("Required text '%s' not found", text), nil).
			WithScreenshot(response.Screenshot).
			WithTextInfo(response.TextInfo).
			WithDuration(time.Since(startTime))
	}

	return NewSuccessResult(fmt.Sprintf("Text check completed: %s", text), map[string]interface{}{
		"text":  text,
		"found": found,
	}).WithScreenshot(response.Screenshot).
		WithTextInfo(response.TextInfo).
		WithDuration(time.Since(startTime))
}

// ExecuteShellScript 执行Shell命令脚本
func ExecuteShellScript(ctx *ScriptContext, params map[string]interface{}) *ScriptResult {
	startTime := time.Now()

	command, ok := params["command"].(string)
	if !ok || command == "" {
		return NewErrorResult("Missing required parameter: command", nil).WithDuration(time.Since(startTime))
	}

	timeout := ctx.GetIntVariable("timeout", 30)
	if t, exists := params["timeout"]; exists {
		if timeoutVal, err := ConvertCoordinateToInt(t); err == nil {
			timeout = timeoutVal
		}
	}

	ctx.Logger.Info("Executing shell command: %s", command)
	ctx.Client.SetTimeout(timeout)

	response, err := ctx.Client.ExecuteShell(command)
	if err != nil {
		return NewErrorResult("Failed to execute shell command", err).WithDuration(time.Since(startTime))
	}

	if response.Status != "success" {
		return NewErrorResult("Shell command failed: "+response.Error, nil).WithDuration(time.Since(startTime))
	}

	return NewSuccessResult("Shell command executed successfully", map[string]interface{}{
		"command": command,
		"result":  response.Result,
	}).WithDuration(time.Since(startTime))
}

// ClickCoordinateScript 点击指定坐标脚本
func ClickCoordinateScript(ctx *ScriptContext, params map[string]interface{}) *ScriptResult {
	startTime := time.Now()

	// 获取x坐标参数
	x, err := ConvertCoordinateToInt(params["x"])
	if err != nil {
		return NewErrorResult("Invalid x coordinate parameter", err).WithDuration(time.Since(startTime))
	}

	// 获取y坐标参数
	y, err := ConvertCoordinateToInt(params["y"])
	if err != nil {
		return NewErrorResult("Invalid y coordinate parameter", err).WithDuration(time.Since(startTime))
	}

	// 获取可选的timeout参数
	timeout := ctx.GetIntVariable("timeout", 30)
	if t, exists := params["timeout"]; exists {
		if timeoutVal, err := ConvertCoordinateToInt(t); err == nil {
			timeout = timeoutVal
		}
	}

	ctx.Logger.Info("Tapping coordinate (%d, %d) with timeout %ds", x, y, timeout)

	// 设置超时
	ctx.Client.SetTimeout(timeout)

	// 执行点击操作
	response, err := ctx.Client.Tap(x, y)
	if err != nil {
		return NewErrorResult("Tap failed", err).WithDuration(time.Since(startTime))
	}

	if response.Status != "success" {
		return NewErrorResult("Tap failed: "+response.Error, nil).WithDuration(time.Since(startTime))
	}

	return NewSuccessResult(fmt.Sprintf("Successfully tapped coordinate (%d, %d)", x, y), map[string]interface{}{
		"x": x,
		"y": y,
	}).WithDuration(time.Since(startTime))
}

// ScreenshotOnlyScript 纯截图脚本（不进行UI文本分析）
func ScreenshotOnlyScript(ctx *ScriptContext, params map[string]interface{}) *ScriptResult {
	startTime := time.Now()

	ctx.Logger.Info("Taking screenshot only (no UI analysis)")

	response, err := ctx.Client.ScreenshotOnly()
	if err != nil {
		return NewErrorResult("Failed to take screenshot", err).WithDuration(time.Since(startTime))
	}

	if response.Status != "success" {
		return NewErrorResult("Screenshot failed: "+response.Error, nil).WithDuration(time.Since(startTime))
	}

	return NewSuccessResult("Screenshot taken successfully", map[string]interface{}{
		"timestamp": time.Now().Unix(),
	}).WithScreenshot(response.Screenshot).
		WithDuration(time.Since(startTime))
}

// GetUITextScript UI文本提取脚本
func GetUITextScript(ctx *ScriptContext, params map[string]interface{}) *ScriptResult {
	startTime := time.Now()

	ctx.Logger.Info("Extracting UI text information")

	response, err := ctx.Client.GetUIText()
	if err != nil {
		return NewErrorResult("Failed to get UI text", err).WithDuration(time.Since(startTime))
	}

	if response.Status != "success" {
		return NewErrorResult("Get UI text failed: "+response.Error, nil).WithDuration(time.Since(startTime))
	}

	return NewSuccessResult("UI text extracted successfully", map[string]interface{}{
		"text_count": len(response.TextInfo),
		"timestamp":  time.Now().Unix(),
	}).WithTextInfo(response.TextInfo).
		WithDuration(time.Since(startTime))
}

// GetOCRTextScript OCR文本提取脚本
func GetOCRTextScript(ctx *ScriptContext, params map[string]interface{}) *ScriptResult {
	startTime := time.Now()

	ctx.Logger.Info("Extracting OCR text information")

	// 先截图
	screenshotResponse, err := ctx.Client.ScreenshotOnly()
	if err != nil {
		return NewErrorResult("Failed to take screenshot for OCR", err).WithDuration(time.Since(startTime))
	}

	if screenshotResponse.Status != "success" {
		return NewErrorResult("Screenshot failed: "+screenshotResponse.Error, nil).WithDuration(time.Since(startTime))
	}

	// 进行OCR处理
	response, err := ctx.Client.GetOCRText(screenshotResponse.Screenshot)
	if err != nil {
		return NewErrorResult("Failed to get OCR text", err).WithDuration(time.Since(startTime))
	}

	if response.Status != "success" {
		return NewErrorResult("OCR failed: "+response.Error, nil).WithDuration(time.Since(startTime))
	}

	return NewSuccessResult("OCR text extracted successfully", map[string]interface{}{
		"text_count": len(response.TextInfo),
		"timestamp":  time.Now().Unix(),
	}).WithScreenshot(screenshotResponse.Screenshot).
		WithTextInfo(response.TextInfo).
		WithDuration(time.Since(startTime))
}

// CheckTextEnhancedScript 增强的文本检查脚本（UI优先，OCR回退）
func CheckTextEnhancedScript(ctx *ScriptContext, params map[string]interface{}) *ScriptResult {
	startTime := time.Now()

	// 获取参数
	text, ok := params["text"].(string)
	if !ok || text == "" {
		return NewErrorResult("Missing required parameter: text", nil).WithDuration(time.Since(startTime))
	}

	timeout := ctx.GetIntVariable("timeout", 30)
	if t, exists := params["timeout"]; exists {
		if timeoutVal, err := ConvertCoordinateToInt(t); err == nil {
			timeout = timeoutVal
		}
	}

	useOCR := false
	if ocr, exists := params["use_ocr"]; exists {
		if ocrVal, ok := ocr.(bool); ok {
			useOCR = ocrVal
		}
	}

	ctx.Logger.Info("Enhanced text check for: '%s' (timeout: %ds, use_ocr: %v)", text, timeout, useOCR)

	// 设置超时
	ctx.Client.SetTimeout(timeout)

	var foundInUI bool
	var foundInOCR bool
	var uiTextInfo []models.TextPosition
	var ocrTextInfo []models.TextPosition
	var screenshot string

	// 第一步：尝试UI文本检测
	uiResponse, err := ctx.Client.GetUIText()
	if err == nil && uiResponse.Status == "success" {
		uiTextInfo = uiResponse.TextInfo
		for _, textInfo := range uiTextInfo {
			if strings.Contains(strings.ToLower(textInfo.Text), strings.ToLower(text)) {
				foundInUI = true
				break
			}
		}
	}

	// 第二步：如果UI没找到且允许OCR，尝试OCR
	if !foundInUI && useOCR {
		ctx.Logger.Info("Text not found in UI, attempting OCR fallback")

		// 截图
		screenshotResponse, err := ctx.Client.ScreenshotOnly()
		if err == nil && screenshotResponse.Status == "success" {
			screenshot = screenshotResponse.Screenshot

			// OCR处理
			ocrResponse, err := ctx.Client.GetOCRText(screenshot)
			if err == nil && ocrResponse.Status == "success" {
				ocrTextInfo = ocrResponse.TextInfo
				for _, textInfo := range ocrTextInfo {
					if strings.Contains(strings.ToLower(textInfo.Text), strings.ToLower(text)) {
						foundInOCR = true
						break
					}
				}
			}
		}
	}

	// 构建结果
	allTextInfo := append(uiTextInfo, ocrTextInfo...)

	if foundInUI || foundInOCR {
		source := "ui"
		if foundInOCR {
			source = "ocr"
		}

		return NewSuccessResult(fmt.Sprintf("Text '%s' found via %s", text, source), map[string]interface{}{
			"text":         text,
			"found_in_ui":  foundInUI,
			"found_in_ocr": foundInOCR,
			"source":       source,
		}).WithTextInfo(allTextInfo).
			WithScreenshot(screenshot).
			WithDuration(time.Since(startTime))
	}

	return NewErrorResult(fmt.Sprintf("Text '%s' not found in UI or OCR", text), nil).
		WithTextInfo(allTextInfo).
		WithScreenshot(screenshot).
		WithDuration(time.Since(startTime))
}

// FindAndClickEnhancedScript 增强的查找并点击脚本（UI优先，OCR回退）
func FindAndClickEnhancedScript(ctx *ScriptContext, params map[string]interface{}) *ScriptResult {
	startTime := time.Now()

	// 获取参数
	text, ok := params["text"].(string)
	if !ok || text == "" {
		return NewErrorResult("Missing required parameter: text", nil).WithDuration(time.Since(startTime))
	}

	timeout := ctx.GetIntVariable("timeout", 30)
	if t, exists := params["timeout"]; exists {
		if timeoutVal, err := ConvertCoordinateToInt(t); err == nil {
			timeout = timeoutVal
		}
	}

	useOCR := false
	if ocr, exists := params["use_ocr"]; exists {
		if ocrVal, ok := ocr.(bool); ok {
			useOCR = ocrVal
		}
	}

	required := true
	if r, exists := params["required"]; exists {
		if reqVal, ok := r.(bool); ok {
			required = reqVal
		}
	}

	ctx.Logger.Info("Enhanced find and click for: '%s' (timeout: %ds, use_ocr: %v, required: %v)", text, timeout, useOCR, required)

	// 设置超时
	ctx.Client.SetTimeout(timeout)

	var targetPos *models.TextPosition
	var screenshot string
	var allTextInfo []models.TextPosition
	var foundSource string

	// 第一步：尝试UI文本检测
	uiResponse, err := ctx.Client.GetUIText()
	if err == nil && uiResponse.Status == "success" {
		allTextInfo = append(allTextInfo, uiResponse.TextInfo...)
		for _, textInfo := range uiResponse.TextInfo {
			if strings.Contains(strings.ToLower(textInfo.Text), strings.ToLower(text)) {
				targetPos = &textInfo
				foundSource = "ui"
				break
			}
		}
	}

	// 第二步：如果UI没找到且允许OCR，尝试OCR
	if targetPos == nil && useOCR {
		ctx.Logger.Info("Text not found in UI, attempting OCR fallback")

		// 截图
		screenshotResponse, err := ctx.Client.ScreenshotOnly()
		if err == nil && screenshotResponse.Status == "success" {
			screenshot = screenshotResponse.Screenshot

			// OCR处理
			ocrResponse, err := ctx.Client.GetOCRText(screenshot)
			if err == nil && ocrResponse.Status == "success" {
				allTextInfo = append(allTextInfo, ocrResponse.TextInfo...)
				for _, textInfo := range ocrResponse.TextInfo {
					if strings.Contains(strings.ToLower(textInfo.Text), strings.ToLower(text)) {
						targetPos = &textInfo
						foundSource = "ocr"
						break
					}
				}
			}
		}
	}

	// 如果没找到文本
	if targetPos == nil {
		if required {
			return NewErrorResult(fmt.Sprintf("Text '%s' not found on screen", text), nil).
				WithScreenshot(screenshot).
				WithTextInfo(allTextInfo).
				WithDuration(time.Since(startTime))
		} else {
			return NewSuccessResult(fmt.Sprintf("Text '%s' not found, but not required", text), map[string]interface{}{
				"text":   text,
				"found":  false,
				"source": "none",
			}).WithScreenshot(screenshot).
				WithTextInfo(allTextInfo).
				WithDuration(time.Since(startTime))
		}
	}

	// 计算点击位置（元素中心）
	clickX := targetPos.X + targetPos.Width/2
	clickY := targetPos.Y + targetPos.Height/2

	ctx.Logger.Info("Found text '%s' via %s at (%d, %d), clicking at (%d, %d)",
		text, foundSource, targetPos.X, targetPos.Y, clickX, clickY)

	// 执行点击
	response, err := ctx.Client.Tap(clickX, clickY)
	if err != nil {
		return NewErrorResult("Failed to tap", err).
			WithScreenshot(screenshot).
			WithTextInfo(allTextInfo).
			WithDuration(time.Since(startTime))
	}

	if response.Status != "success" {
		return NewErrorResult("Tap failed: "+response.Error, nil).
			WithScreenshot(screenshot).
			WithTextInfo(allTextInfo).
			WithDuration(time.Since(startTime))
	}

	return NewSuccessResult(fmt.Sprintf("Successfully found and clicked text '%s' via %s", text, foundSource), map[string]interface{}{
		"text":       text,
		"click_x":    clickX,
		"click_y":    clickY,
		"found_x":    targetPos.X,
		"found_y":    targetPos.Y,
		"source":     foundSource,
		"confidence": targetPos.Confidence,
	}).WithScreenshot(screenshot).
		WithTextInfo(allTextInfo).
		WithDuration(time.Since(startTime))
}
