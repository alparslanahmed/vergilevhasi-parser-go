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

// Real PDF test cases
var realPDFTests = []struct {
	path        string
	expectedVKN string
}{
	{"1.pdf", "1222153986"},
	{"4.pdf", "4610487832"},
}

func TestRealPDFVKNExtraction(t *testing.T) {
	for _, tc := range realPDFTests {
		t.Run(tc.path, func(t *testing.T) {
			if _, err := os.Stat(tc.path); os.IsNotExist(err) {
				t.Skipf("PDF file not found: %s", tc.path)
				return
			}

			parser, err := NewOCRParser()
			if err != nil {
				t.Fatalf("Failed to create OCR parser: %v", err)
			}
			parser.SetOCRDebug(true)

			vkn, err := parser.ExtractVKNFromPDFWithImage(tc.path)
			if err != nil {
				t.Logf("ExtractVKNFromPDFWithImage error: %v", err)
			}

			if vkn != tc.expectedVKN {
				t.Errorf("VKN mismatch for %s: got %s, expected %s", tc.path, vkn, tc.expectedVKN)
			} else {
				t.Logf("✓ VKN correctly extracted: %s", vkn)
			}
		})
	}
}

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

func TestIsValidVKN(t *testing.T) {
	tests := []struct {
		name  string
		vkn   string
		valid bool
	}{
		{
			name:  "Valid VKN from test PDF 1",
			vkn:   "1222153986",
			valid: true,
		},
		{
			name:  "Valid VKN from test PDF 4",
			vkn:   "8589706200",
			valid: true,
		},
		{
			name:  "Valid 10 digit number",
			vkn:   "1234567890",
			valid: true,
		},
		{
			name:  "Invalid - starts with 0",
			vkn:   "0123456789",
			valid: false,
		},
		{
			name:  "Invalid - too short",
			vkn:   "123456789",
			valid: false,
		},
		{
			name:  "Invalid - too long",
			vkn:   "12345678901",
			valid: false,
		},
		{
			name:  "Invalid - contains letters",
			vkn:   "123456789A",
			valid: false,
		},
		{
			name:  "Invalid - looks like date DDMMYYYY",
			vkn:   "1501202300",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidVKN(tt.vkn)
			if got != tt.valid {
				t.Errorf("isValidVKN(%s) = %v, want %v", tt.vkn, got, tt.valid)
			}
		})
	}
}
