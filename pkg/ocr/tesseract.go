package ocr

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"mq_adb/pkg/models"

	"github.com/otiai10/gosseract/v2"
)

// TesseractProvider implements OCR using Tesseract
type TesseractProvider struct {
	client *gosseract.Client
}

// NewTesseractProvider creates a new Tesseract provider
func NewTesseractProvider() (*TesseractProvider, error) {
	client := gosseract.NewClient()

	// Set languages for multi-language support
	// eng: English, chi_sim: Simplified Chinese, jpn: Japanese, kor: Korean
	err := client.SetLanguage("eng+chi_sim+jpn+kor")
	if err != nil {
		log.Printf("Warning: Failed to set multi-language support, falling back to English: %v", err)
		// Fallback to English only
		err = client.SetLanguage("eng")
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("failed to initialize Tesseract OCR engine: %v", err)
		}
	}

	// Set page segmentation mode for better text detection
	// PSM_AUTO: Fully automatic page segmentation
	client.SetPageSegMode(gosseract.PSM_AUTO)

	// Note: SetOCREngineMode may not be available in all versions
	// Commented out for compatibility
	// client.SetOCREngineMode(gosseract.OEM_LSTM_ONLY)

	return &TesseractProvider{
		client: client,
	}, nil
}

// ProcessImage extracts text from an image using Tesseract OCR
func (tp *TesseractProvider) ProcessImage(imageData []byte) ([]models.TextPosition, error) {
	if tp.client == nil {
		return nil, fmt.Errorf("Tesseract OCR engine not initialized")
	}

	// Set image data
	err := tp.client.SetImageFromBytes(imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to set image data: %v", err)
	}

	// Get bounding boxes with confidence scores
	boxes, err := tp.client.GetBoundingBoxes(gosseract.RIL_WORD)
	if err != nil {
		return nil, fmt.Errorf("failed to get bounding boxes: %v", err)
	}

	var textPositions []models.TextPosition

	for _, box := range boxes {
		// Filter out low confidence results (below 30%)
		if box.Confidence < 30.0 {
			continue
		}

		// Clean up the extracted text
		text := strings.TrimSpace(box.Word)
		if text == "" {
			continue
		}

		// Skip very short single characters unless they are meaningful
		if len(text) == 1 && !isValidSingleCharTesseract(text) {
			continue
		}

		textPosition := models.TextPosition{
			Text:       text,
			X:          box.Box.Min.X,
			Y:          box.Box.Min.Y,
			Width:      box.Box.Max.X - box.Box.Min.X,
			Height:     box.Box.Max.Y - box.Box.Min.Y,
			Confidence: box.Confidence,
			Source:     "tesseract",
		}

		textPositions = append(textPositions, textPosition)
	}

	log.Printf("Tesseract OCR extracted %d text elements", len(textPositions))

	return textPositions, nil
}

// SetLanguages updates the language configuration for Tesseract
func (tp *TesseractProvider) SetLanguages(languages []string) error {
	if tp.client == nil {
		return fmt.Errorf("Tesseract OCR engine not initialized")
	}

	langString := strings.Join(languages, "+")
	return tp.client.SetLanguage(langString)
}

// GetSupportedLanguages returns the list of supported languages for Tesseract
func (tp *TesseractProvider) GetSupportedLanguages() []string {
	return []string{"eng", "chi_sim", "jpn", "kor"}
}

// Close releases Tesseract OCR engine resources
func (tp *TesseractProvider) Close() error {
	if tp.client != nil {
		return tp.client.Close()
	}
	return nil
}

// GetName returns the provider name
func (tp *TesseractProvider) GetName() string {
	return "Tesseract"
}

// isValidSingleCharTesseract checks if a single character is meaningful for Tesseract
func isValidSingleCharTesseract(char string) bool {
	// Allow digits, letters, and common meaningful symbols
	matched, _ := regexp.MatchString(`[0-9a-zA-Z\u4e00-\u9fff\u3040-\u309f\u30a0-\u30ff\uac00-\ud7af+\-=<>!@#$%&*()]`, char)
	return matched
}
