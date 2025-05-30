package ocr

import (
	"fmt"
	"log"
	"strings"

	"mq_adb/pkg/models"
)

// 初始化 OCR 提供者
func init() {
	log.Println("🔧 Initializing OCR providers...")

	// 注册 Tesseract 提供者（目前唯一支持的引擎）
	if tesseractProvider, err := NewTesseractProvider(); err == nil {
		GlobalOCRManager.RegisterProvider(EngineTypeTesseract, tesseractProvider)
		GlobalOCRManager.SetDefaultEngine(EngineTypeTesseract)
		log.Println("✅ Tesseract provider registered successfully")
	} else {
		log.Printf("❌ Tesseract provider registration failed: %v", err)
	}

	// 显示最终状态
	if provider, err := GlobalOCRManager.GetDefaultProvider(); err == nil {
		log.Printf("🚀 OCR system ready. Default engine: %s", provider.GetName())
	} else {
		log.Printf("❌ No OCR providers available: %v", err)
	}
}

// ProcessImage processes an image for OCR (convenience function using default engine)
func ProcessImage(imageData []byte, languages string) ([]models.TextPosition, error) {
	return GlobalOCRManager.ProcessImage(imageData, languages)
}

// ProcessImageWithEngine processes an image using a specific OCR engine
func ProcessImageWithEngine(imageData []byte, engineType string, languages string) ([]models.TextPosition, error) {
	var engine OCREngineType
	switch strings.ToLower(engineType) {
	case "tesseract":
		engine = EngineTypeTesseract
	default:
		return nil, fmt.Errorf("unsupported OCR engine: %s (only 'tesseract' is currently supported)", engineType)
	}

	return GlobalOCRManager.ProcessImageWithEngine(imageData, engine, languages)
}

// GetAvailableEngines returns list of available OCR engines
func GetAvailableEngines() []string {
	var engines []string
	for engineType := range GlobalOCRManager.providers {
		engines = append(engines, string(engineType))
	}
	return engines
}

// GetEngineStatus returns status information for each engine
func GetEngineStatus() map[string]interface{} {
	status := make(map[string]interface{})

	for engineType, provider := range GlobalOCRManager.providers {
		engineStatus := map[string]interface{}{
			"name":                provider.GetName(),
			"supported_languages": provider.GetSupportedLanguages(),
			"available":           true,
		}
		status[string(engineType)] = engineStatus
	}

	// 添加默认引擎信息
	status["default_engine"] = string(GlobalOCRManager.defaultEngine)

	return status
}

// SetDefaultEngine sets the default OCR engine
func SetDefaultEngine(engineType string) error {
	var engine OCREngineType
	switch strings.ToLower(engineType) {
	case "tesseract":
		engine = EngineTypeTesseract
	default:
		return fmt.Errorf("unsupported OCR engine: %s (only 'tesseract' is currently supported)", engineType)
	}

	if _, exists := GlobalOCRManager.GetProvider(engine); !exists {
		return fmt.Errorf("OCR engine %s is not available", engineType)
	}

	GlobalOCRManager.SetDefaultEngine(engine)
	log.Printf("🔧 Default OCR engine set to: %s", engineType)
	return nil
}
