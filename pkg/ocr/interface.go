package ocr

import (
	"fmt"
	"log"
	"strings"

	"mq_adb/pkg/models"
)

// OCRProvider defines the interface for OCR engines
type OCRProvider interface {
	// ProcessImage processes an image and returns text positions
	ProcessImage(imageData []byte) ([]models.TextPosition, error)

	// SetLanguages sets the languages for OCR recognition
	SetLanguages(languages []string) error

	// GetSupportedLanguages returns supported language codes
	GetSupportedLanguages() []string

	// Close releases resources
	Close() error

	// GetName returns the provider name
	GetName() string
}

// OCREngineType represents different OCR engine types
type OCREngineType string

const (
	EngineTypeTesseract OCREngineType = "tesseract"
	EngineTypePaddleOCR OCREngineType = "paddleocr"
)

// OCRManager manages multiple OCR providers
type OCRManager struct {
	providers     map[OCREngineType]OCRProvider
	defaultEngine OCREngineType
}

// NewOCRManager creates a new OCR manager
func NewOCRManager() *OCRManager {
	return &OCRManager{
		providers:     make(map[OCREngineType]OCRProvider),
		defaultEngine: EngineTypePaddleOCR, // 默认使用 PaddleOCR
	}
}

// RegisterProvider registers an OCR provider
func (m *OCRManager) RegisterProvider(engineType OCREngineType, provider OCRProvider) {
	m.providers[engineType] = provider
}

// GetProvider gets an OCR provider by type
func (m *OCRManager) GetProvider(engineType OCREngineType) (OCRProvider, bool) {
	provider, exists := m.providers[engineType]
	return provider, exists
}

// GetDefaultProvider gets the default OCR provider
func (m *OCRManager) GetDefaultProvider() (OCRProvider, error) {
	provider, exists := m.providers[m.defaultEngine]
	if !exists {
		// 如果默认引擎不可用，尝试使用其他可用的引擎
		for _, p := range m.providers {
			return p, nil
		}
		return nil, fmt.Errorf("no OCR providers available")
	}
	return provider, nil
}

// SetDefaultEngine sets the default OCR engine
func (m *OCRManager) SetDefaultEngine(engineType OCREngineType) {
	m.defaultEngine = engineType
}

// ProcessImage processes an image using the default OCR engine
func (m *OCRManager) ProcessImage(imageData []byte, languages string) ([]models.TextPosition, error) {
	provider, err := m.GetDefaultProvider()
	if err != nil {
		return nil, err
	}

	// Set languages if provided
	if languages != "" {
		langList := strings.Split(languages, "+")
		if err := provider.SetLanguages(langList); err != nil {
			log.Printf("Warning: failed to set languages %s: %v", languages, err)
		}
	}

	return provider.ProcessImage(imageData)
}

// ProcessImageWithEngine processes an image using a specific OCR engine
func (m *OCRManager) ProcessImageWithEngine(imageData []byte, engineType OCREngineType, languages string) ([]models.TextPosition, error) {
	provider, exists := m.GetProvider(engineType)
	if !exists {
		return nil, fmt.Errorf("OCR engine %s not available", engineType)
	}

	// Set languages if provided
	if languages != "" {
		langList := strings.Split(languages, "+")
		if err := provider.SetLanguages(langList); err != nil {
			log.Printf("Warning: failed to set languages %s: %v", languages, err)
		}
	}

	return provider.ProcessImage(imageData)
}

// Close closes all OCR providers
func (m *OCRManager) Close() error {
	for _, provider := range m.providers {
		if err := provider.Close(); err != nil {
			log.Printf("Error closing OCR provider %s: %v", provider.GetName(), err)
		}
	}
	return nil
}

// Global OCR manager instance
var GlobalOCRManager *OCRManager

func init() {
	GlobalOCRManager = NewOCRManager()
}
