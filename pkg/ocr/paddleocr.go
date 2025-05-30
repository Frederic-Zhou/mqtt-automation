package ocr

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"mq_adb/pkg/models"
)

// PaddleOCRProvider implements OCR using PaddleOCR
type PaddleOCRProvider struct {
	pythonPath    string
	scriptPath    string
	languages     []string
	serverMode    bool
	serverURL     string
	serverProcess *exec.Cmd
}

// PaddleOCRResult represents the result from PaddleOCR
type PaddleOCRResult struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Box        [][]int `json:"box"` // [[x1,y1], [x2,y2], [x3,y3], [x4,y4]]
}

// NewPaddleOCRProvider creates a new PaddleOCR provider
func NewPaddleOCRProvider() (*PaddleOCRProvider, error) {
	provider := &PaddleOCRProvider{
		languages:  []string{"ch", "en"}, // 默认中英文
		serverMode: true,                 // 默认使用服务器模式以提高性能
		serverURL:  "http://localhost:8868",
	}

	// 尝试找到 Python 路径
	pythonPath, err := findPython()
	if err != nil {
		return nil, fmt.Errorf("Python not found: %v", err)
	}
	provider.pythonPath = pythonPath

	// 创建 PaddleOCR 脚本
	scriptPath, err := provider.createPaddleOCRScript()
	if err != nil {
		return nil, fmt.Errorf("failed to create PaddleOCR script: %v", err)
	}
	provider.scriptPath = scriptPath

	// 验证 PaddleOCR 是否安装
	if err := provider.checkPaddleOCRInstallation(); err != nil {
		return nil, fmt.Errorf("PaddleOCR not properly installed: %v", err)
	}

	// 启动服务器模式（如果启用）
	if provider.serverMode {
		if err := provider.startServer(); err != nil {
			log.Printf("Failed to start PaddleOCR server, falling back to direct mode: %v", err)
			provider.serverMode = false
		}
	}

	return provider, nil
}

// findPython finds Python executable
func findPython() (string, error) {
	candidates := []string{"python3", "python", "/usr/bin/python3", "/usr/local/bin/python3"}

	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("Python executable not found")
}

// checkPaddleOCRInstallation checks if PaddleOCR is installed
func (p *PaddleOCRProvider) checkPaddleOCRInstallation() error {
	cmd := exec.Command(p.pythonPath, "-c", "import paddleocr; print('PaddleOCR available')")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("PaddleOCR not installed: %v\nOutput: %s\n\nTo install PaddleOCR, run:\npip install paddleocr", err, string(output))
	}
	return nil
}

// createPaddleOCRScript creates the Python script for OCR processing
func (p *PaddleOCRProvider) createPaddleOCRScript() (string, error) {
	scriptContent := `#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
PaddleOCR processing script for Go integration
"""
import sys
import json
import base64
import io
import os
from PIL import Image
import numpy as np

try:
    from paddleocr import PaddleOCR
except ImportError:
    print(json.dumps({"error": "PaddleOCR not installed. Run: pip install paddleocr"}))
    sys.exit(1)

def process_image(image_data_b64, languages="ch,en"):
    """Process image with PaddleOCR"""
    try:
        # Initialize OCR
        lang_list = languages.split(',')
        ocr = PaddleOCR(
            use_angle_cls=True,  # 使用角度分类器
            lang=lang_list[0] if lang_list else 'ch',  # 主要语言
            show_log=False
        )
        
        # Decode base64 image
        image_data = base64.b64decode(image_data_b64)
        image = Image.open(io.BytesIO(image_data))
        
        # Convert to numpy array
        img_array = np.array(image)
        
        # Perform OCR
        result = ocr.ocr(img_array, cls=True)
        
        # Parse results
        text_positions = []
        if result and result[0]:
            for line in result[0]:
                box = line[0]  # [[x1,y1], [x2,y2], [x3,y3], [x4,y4]]
                text_info = line[1]  # (text, confidence)
                text = text_info[0]
                confidence = text_info[1] * 100  # Convert to percentage
                
                # Calculate bounding rectangle
                x_coords = [point[0] for point in box]
                y_coords = [point[1] for point in box]
                x = int(min(x_coords))
                y = int(min(y_coords))
                width = int(max(x_coords) - min(x_coords))
                height = int(max(y_coords) - min(y_coords))
                
                text_positions.append({
                    "text": text.strip(),
                    "x": x,
                    "y": y,
                    "width": width,
                    "height": height,
                    "confidence": confidence,
                    "source": "paddleocr",
                    "box": box
                })
        
        return {
            "success": True,
            "text_positions": text_positions,
            "total_found": len(text_positions)
        }
        
    except Exception as e:
        return {
            "success": False,
            "error": str(e),
            "text_positions": []
        }

def main():
    if len(sys.argv) < 2:
        print(json.dumps({"error": "Usage: script.py <base64_image_data> [languages]"}))
        sys.exit(1)
    
    image_data_b64 = sys.argv[1]
    languages = sys.argv[2] if len(sys.argv) > 2 else "ch,en"
    
    result = process_image(image_data_b64, languages)
    print(json.dumps(result))

if __name__ == "__main__":
    main()
`

	// 创建临时目录存放脚本
	tempDir := os.TempDir()
	scriptPath := filepath.Join(tempDir, "paddleocr_processor.py")

	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		return "", err
	}

	return scriptPath, nil
}

// startServer starts PaddleOCR server (future enhancement)
func (p *PaddleOCRProvider) startServer() error {
	// 服务器模式的实现可以在未来添加
	// 这里暂时禁用服务器模式
	p.serverMode = false
	return nil
}

// ProcessImage processes an image using PaddleOCR
func (p *PaddleOCRProvider) ProcessImage(imageData []byte) ([]models.TextPosition, error) {
	if p.serverMode {
		return p.processImageViaServer(imageData)
	}
	return p.processImageDirect(imageData)
}

// processImageDirect processes image by calling Python script directly
func (p *PaddleOCRProvider) processImageDirect(imageData []byte) ([]models.TextPosition, error) {
	// Convert image data to base64
	imageB64 := base64.StdEncoding.EncodeToString(imageData)

	// Prepare languages
	languages := strings.Join(p.languages, ",")

	// Execute Python script
	cmd := exec.Command(p.pythonPath, p.scriptPath, imageB64, languages)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("PaddleOCR script failed: %v\nStderr: %s", err, stderr.String())
	}

	// Parse JSON result
	var result struct {
		Success       bool                     `json:"success"`
		Error         string                   `json:"error,omitempty"`
		TextPositions []map[string]interface{} `json:"text_positions"`
		TotalFound    int                      `json:"total_found"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse PaddleOCR result: %v\nOutput: %s", err, stdout.String())
	}

	if !result.Success {
		return nil, fmt.Errorf("PaddleOCR processing failed: %s", result.Error)
	}

	// Convert to models.TextPosition
	var textPositions []models.TextPosition
	for _, tp := range result.TextPositions {
		text, _ := tp["text"].(string)
		x, _ := tp["x"].(float64)
		y, _ := tp["y"].(float64)
		width, _ := tp["width"].(float64)
		height, _ := tp["height"].(float64)
		confidence, _ := tp["confidence"].(float64)
		source, _ := tp["source"].(string)

		// 过滤低置信度和空文本
		if confidence < 30.0 || strings.TrimSpace(text) == "" {
			continue
		}

		// 过滤单字符（除非是有意义的字符）
		if len(text) == 1 && !isValidSingleChar(text) {
			continue
		}

		textPosition := models.TextPosition{
			Text:       strings.TrimSpace(text),
			X:          int(x),
			Y:          int(y),
			Width:      int(width),
			Height:     int(height),
			Confidence: confidence,
			Source:     source,
		}

		textPositions = append(textPositions, textPosition)
	}

	log.Printf("PaddleOCR extracted %d text elements", len(textPositions))
	return textPositions, nil
}

// processImageViaServer processes image via HTTP server (future enhancement)
func (p *PaddleOCRProvider) processImageViaServer(imageData []byte) ([]models.TextPosition, error) {
	// 服务器模式实现（未来增强）
	return p.processImageDirect(imageData)
}

// SetLanguages sets the languages for OCR
func (p *PaddleOCRProvider) SetLanguages(languages []string) error {
	// 映射语言代码
	mappedLangs := make([]string, 0, len(languages))
	for _, lang := range languages {
		mapped := mapLanguageCode(lang)
		if mapped != "" {
			mappedLangs = append(mappedLangs, mapped)
		}
	}

	if len(mappedLangs) == 0 {
		mappedLangs = []string{"ch", "en"} // 默认中英文
	}

	p.languages = mappedLangs
	return nil
}

// mapLanguageCode maps language codes to PaddleOCR format
func mapLanguageCode(lang string) string {
	langMap := map[string]string{
		"eng":      "en",
		"chi_sim":  "ch",
		"jpn":      "japan",
		"kor":      "korean",
		"en":       "en",
		"ch":       "ch",
		"chinese":  "ch",
		"english":  "en",
		"japanese": "japan",
		"korean":   "korean",
	}

	if mapped, exists := langMap[lang]; exists {
		return mapped
	}
	return lang // 返回原始语言代码
}

// GetSupportedLanguages returns supported language codes
func (p *PaddleOCRProvider) GetSupportedLanguages() []string {
	return []string{
		"en", "ch", "ta", "te", "ka", "thai", "la", "ar", "hi", "ug", "fa",
		"ur", "rs", "oc", "rsc", "bg", "uk", "be", "te", "kn", "ch_tra",
		"hi", "mr", "ne", "mai", "ang", "bh", "mai", "gom", "sa", "new",
		"as", "mni", "ml", "or", "pa", "gu", "ta", "te", "kn", "japan",
		"korean", "it", "xi", "pu", "ru", "da", "no", "is", "fo", "lv",
		"lt", "pl", "cs", "sk", "sl", "hr", "bs", "sr", "mk", "bg",
		"ro", "hu", "et", "fi", "sv", "fr", "de", "es", "pt", "it",
		"ca", "eu", "gl", "cy", "ga", "mt", "sq", "az", "uz", "ky",
		"kk", "mn", "tg", "sah", "bua", "chu", "ava", "dar", "kbd",
		"lez", "tab",
	}
}

// Close releases resources
func (p *PaddleOCRProvider) Close() error {
	// 停止服务器进程（如果有）
	if p.serverProcess != nil {
		if err := p.serverProcess.Process.Kill(); err != nil {
			log.Printf("Failed to kill PaddleOCR server process: %v", err)
		}
		p.serverProcess = nil
	}

	// 清理临时脚本文件
	if p.scriptPath != "" {
		if err := os.Remove(p.scriptPath); err != nil {
			log.Printf("Failed to remove PaddleOCR script: %v", err)
		}
	}

	return nil
}

// GetName returns the provider name
func (p *PaddleOCRProvider) GetName() string {
	return "PaddleOCR"
}

// isValidSingleChar checks if a single character is meaningful (reused from tesseract)
func isValidSingleChar(char string) bool {
	// Allow digits, letters, and common meaningful symbols
	matched, _ := regexp.MatchString(`[0-9a-zA-Z\u4e00-\u9fff\u3040-\u309f\u30a0-\u30ff\uac00-\ud7af+\-=<>!@#$%&*()]`, char)
	return matched
}
