//go:build ocr
// +build ocr

package vergilevhasi

import (
	"os"
	"testing"
)

// testPDFPath is the path to the test PDF file.
// To run these tests, place a sample tax plate PDF at this path.
const testPDFPath = "testdata/sample_vergi_levhasi.pdf"

// expectedTestVKN is the expected VKN from the test PDF.
// Update this value to match the VKN in your test PDF.
const expectedTestVKN = "1234567890"

func TestExtractVKNWithImgconv(t *testing.T) {
	// Skip if no PDF file is available
	if _, err := os.Stat(testPDFPath); os.IsNotExist(err) {
		t.Skip("No test PDF file available at " + testPDFPath)
	}

	parser, err := NewOCRParser()
	if err != nil {
		t.Fatalf("Failed to create OCR parser: %v", err)
	}

	// Test the full extraction method
	vkn, err := parser.ExtractVKNFromPDFWithImage(testPDFPath)
	if err != nil {
		t.Fatalf("Error extracting VKN: %v", err)
	}

	if vkn != expectedTestVKN {
		t.Errorf("VKN mismatch. Got: %s, Expected: %s", vkn, expectedTestVKN)
	}
}

// TestExtractVKNWithImgconvDirect directly tests the imgconv path bypassing text extraction
func TestExtractVKNWithImgconvDirect(t *testing.T) {
	// Skip if no PDF file is available
	if _, err := os.Stat(testPDFPath); os.IsNotExist(err) {
		t.Skip("No test PDF file available at " + testPDFPath)
	}

	parser, err := NewOCRParser()
	if err != nil {
		t.Fatalf("Failed to create OCR parser: %v", err)
	}

	// Read PDF file
	pdfFile, err := os.Open(testPDFPath)
	if err != nil {
		t.Fatalf("Failed to open PDF: %v", err)
	}
	defer pdfFile.Close()

	// Directly test the imgconv reader method (bypasses text extraction)
	vkn, err := parser.ExtractVKNFromPDFReaderWithImage(pdfFile)
	if err != nil {
		t.Fatalf("Error extracting VKN with imgconv: %v", err)
	}

	if vkn != expectedTestVKN {
		t.Errorf("VKN mismatch. Got: %s, Expected: %s", vkn, expectedTestVKN)
	}
}

func TestExtractVKNFromPDFBytes(t *testing.T) {
	// Skip if no PDF file is available
	if _, err := os.Stat(testPDFPath); os.IsNotExist(err) {
		t.Skip("No test PDF file available at " + testPDFPath)
	}

	pdfData, err := os.ReadFile(testPDFPath)
	if err != nil {
		t.Fatalf("Failed to read PDF: %v", err)
	}

	parser, err := NewOCRParser()
	if err != nil {
		t.Fatalf("Failed to create OCR parser: %v", err)
	}

	vkn, err := parser.ExtractVKNFromPDFBytes(pdfData)
	if err != nil {
		t.Fatalf("Error extracting VKN from bytes: %v", err)
	}

	if vkn != expectedTestVKN {
		t.Errorf("VKN mismatch. Got: %s, Expected: %s", vkn, expectedTestVKN)
	}
}
