package ocr

import (
	"fmt"
	"log"
)

// InitializeOCRProviders initializes all available OCR providers
func InitializeOCRProviders() error {
	log.Println("Initializing OCR providers...")

	// 尝试初始化 PaddleOCR (优先)
	if paddleProvider, err := NewPaddleOCRProvider(); err == nil {
		GlobalOCRManager.RegisterProvider(EngineTypePaddleOCR, paddleProvider)
		GlobalOCRManager.SetDefaultEngine(EngineTypePaddleOCR)
		log.Println("✅ PaddleOCR provider initialized successfully")
	} else {
		log.Printf("⚠️  PaddleOCR provider failed to initialize: %v", err)
	}

	// 尝试初始化 Tesseract (备用)
	if tesseractProvider, err := NewTesseractProvider(); err == nil {
		GlobalOCRManager.RegisterProvider(EngineTypeTesseract, tesseractProvider)
		// 如果 PaddleOCR 不可用，使用 Tesseract 作为默认
		if _, exists := GlobalOCRManager.GetProvider(EngineTypePaddleOCR); !exists {
			GlobalOCRManager.SetDefaultEngine(EngineTypeTesseract)
		}
		log.Println("✅ Tesseract provider initialized successfully")
	} else {
		log.Printf("⚠️  Tesseract provider failed to initialize: %v", err)
	}

	// 检查是否至少有一个 OCR 引擎可用
	if _, err := GlobalOCRManager.GetDefaultProvider(); err != nil {
		return fmt.Errorf("no OCR providers available: %v", err)
	}

	// 显示当前使用的引擎
	defaultProvider, _ := GlobalOCRManager.GetDefaultProvider()
	log.Printf("🚀 OCR system ready. Default engine: %s", defaultProvider.GetName())

	return nil
}

// GetAvailableEnginesSimple returns a list of available OCR engines (simple format)
func GetAvailableEnginesSimple() []string {
	engines := make([]string, 0)

	if _, exists := GlobalOCRManager.GetProvider(EngineTypePaddleOCR); exists {
		engines = append(engines, "paddleocr")
	}

	if _, exists := GlobalOCRManager.GetProvider(EngineTypeTesseract); exists {
		engines = append(engines, "tesseract")
	}

	return engines
}

// GetDefaultEngine returns the name of the default OCR engine
func GetDefaultEngine() string {
	if provider, err := GlobalOCRManager.GetDefaultProvider(); err == nil {
		return provider.GetName()
	}
	return "none"
}

// CleanupOCR cleans up OCR resources
func CleanupOCR() error {
	return GlobalOCRManager.Close()
}
