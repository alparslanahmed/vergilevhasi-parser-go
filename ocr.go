/*
Package vergilevhasi OCR module provides digit recognition for VKN extraction.

This is a ZERO-DEPENDENCY implementation that works without:
- ONNX Runtime
- TensorFlow Lite
- Tesseract
- Any external tools

It uses:
- Pure Go image processing
- Built-in PDF text/image extraction from the pdfcpu library
- Feature-based digit recognition with a trained classifier

Usage:

	parser, _ := vergilevhasi.NewOCRParser()
	vkn, err := parser.ExtractVKNFromImage("image.png")
*/
package vergilevhasi

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"

	_ "image/gif"
	_ "image/jpeg"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/oned"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// OCRParser provides OCR capabilities for VKN extraction
// Zero external dependencies - works with pure Go
type OCRParser struct {
	*Parser
	classifier *DigitClassifier
	debug      bool
}

// NewOCRParser creates a new OCR parser with zero dependencies
func NewOCRParser() (*OCRParser, error) {
	return &OCRParser{
		Parser:     NewParser(),
		classifier: NewDigitClassifier(),
		debug:      false,
	}, nil
}

// Close releases resources (no-op for pure Go implementation)
func (p *OCRParser) Close() error {
	return nil
}

// SetOCRDebug enables debug output
func (p *OCRParser) SetOCRDebug(debug bool) {
	p.debug = debug
}

// ExtractVKNFromPDFWithImage extracts VKN from a PDF by extracting embedded images and scanning barcodes
// Uses pdfcpu for image extraction (pure Go, no external dependencies)
func (p *OCRParser) ExtractVKNFromPDFWithImage(data []byte) (string, error) {
	if p.debug {
		fmt.Println("Extracting images from PDF using pdfcpu...")
	}

	vkn, err := p.ExtractVKNFromPDFBytes(data)

	return vkn, err
}

// extractAllPDFImages extracts all images embedded in a PDF using pdfcpu's native extraction
func (p *OCRParser) extractAllPDFImages(pdfData []byte) (images []image.Image, err error) {
	// Recover from any panics in pdfcpu
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while extracting PDF images: %v", r)
			images = nil
		}
	}()

	// Create a reader from the data
	rs := bytes.NewReader(pdfData)

	// Create pdfcpu configuration
	conf := model.NewDefaultConfiguration()

	// Use api.ExtractImagesRaw to get all images
	pageImages, err := api.ExtractImagesRaw(rs, nil, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to extract images: %w", err)
	}

	if p.debug {
		fmt.Printf("Found images on %d pages\n", len(pageImages))
	}

	// Process images from all pages
	for pageNr, imgMap := range pageImages {
		if p.debug {
			fmt.Printf("Page %d: found %d images\n", pageNr+1, len(imgMap))
		}

		for objNr, pdfImage := range imgMap {
			if p.debug {
				fmt.Printf("Image obj %d: type=%s, %dx%d, bpc=%d, comp=%d\n",
					objNr, pdfImage.FileType, pdfImage.Width, pdfImage.Height, pdfImage.Bpc, pdfImage.Comp)
			}
			// Decode the image from the pdfcpu Image reader
			img, err := p.decodePDFCPUImage(pdfImage)
			if err != nil {
				if p.debug {
					fmt.Printf("Failed to decode image obj %d: %v\n", objNr, err)
				}
				continue
			}
			images = append(images, img)
		}
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("no images found in PDF")
	}

	return images, nil
}

// decodePDFCPUImage decodes a pdfcpu model.Image to a Go image.Image
func (p *OCRParser) decodePDFCPUImage(pdfImage model.Image) (image.Image, error) {
	// Read all data from the image reader
	data, err := io.ReadAll(pdfImage)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	// Create a reader from the data
	reader := bytes.NewReader(data)

	// Try to decode based on the FileType
	fileType := strings.ToLower(pdfImage.FileType)

	switch fileType {
	case "png":
		return png.Decode(reader)
	case "jpg", "jpeg":
		return decodeJPEG(reader)
	case "gif":
		return decodeGIF(reader)
	default:
		// Try standard image.Decode which handles registered formats
		img, _, err := image.Decode(reader)
		if err != nil {
			// If standard decode fails, try to decode as raw image data
			return p.decodeRawImageData(data, pdfImage)
		}
		return img, nil
	}
}

// decodeJPEG decodes JPEG image data
func decodeJPEG(r io.Reader) (image.Image, error) {
	// image/jpeg is already registered via _ "image/jpeg" import
	img, _, err := image.Decode(r)
	return img, err
}

// decodeGIF decodes GIF image data
func decodeGIF(r io.Reader) (image.Image, error) {
	// image/gif is already registered via _ "image/gif" import
	img, _, err := image.Decode(r)
	return img, err
}

// decodeRawImageData attempts to decode raw image data based on PDF image properties
func (p *OCRParser) decodeRawImageData(data []byte, pdfImage model.Image) (image.Image, error) {
	width := pdfImage.Width
	height := pdfImage.Height
	bpc := pdfImage.Bpc   // bits per component
	comp := pdfImage.Comp // number of color components

	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid image dimensions: %dx%d", width, height)
	}

	// Calculate expected data size
	expectedSize := width * height * comp * bpc / 8
	if len(data) < expectedSize {
		// Try image.Decode as fallback
		img, _, err := image.Decode(bytes.NewReader(data))
		if err == nil {
			return img, nil
		}
		return nil, fmt.Errorf("data size mismatch: got %d, expected %d", len(data), expectedSize)
	}

	// Create image based on color space
	switch {
	case pdfImage.Cs == "DeviceGray" || comp == 1:
		// Grayscale image
		img := image.NewGray(image.Rect(0, 0, width, height))
		if bpc == 8 {
			copy(img.Pix, data[:width*height])
		} else if bpc == 1 {
			// 1-bit image (black and white)
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					byteIdx := (y*width + x) / 8
					bitIdx := 7 - ((y*width + x) % 8)
					if byteIdx < len(data) {
						bit := (data[byteIdx] >> bitIdx) & 1
						if bit == 0 {
							img.SetGray(x, y, color.Gray{0}) // black
						} else {
							img.SetGray(x, y, color.Gray{255}) // white
						}
					}
				}
			}
		}
		return img, nil

	case pdfImage.Cs == "DeviceRGB" || comp == 3:
		// RGB image
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				idx := (y*width + x) * 3
				if idx+2 < len(data) {
					img.SetRGBA(x, y, color.RGBA{data[idx], data[idx+1], data[idx+2], 255})
				}
			}
		}
		return img, nil

	case pdfImage.Cs == "DeviceCMYK" || comp == 4:
		// CMYK image - convert to RGBA
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				idx := (y*width + x) * 4
				if idx+3 < len(data) {
					c, m, yk, k := data[idx], data[idx+1], data[idx+2], data[idx+3]
					// Convert CMYK to RGB
					r := 255 - min(255, int(c)+int(k))
					g := 255 - min(255, int(m)+int(k))
					b := 255 - min(255, int(yk)+int(k))
					img.SetRGBA(x, y, color.RGBA{uint8(r), uint8(g), uint8(b), 255})
				}
			}
		}
		return img, nil

	default:
		// Try standard decode as last resort
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("unsupported color space: %s with %d components", pdfImage.Cs, comp)
		}
		return img, nil
	}
}

// upscaleImage upscales an image by the given factor using nearest-neighbor interpolation
func (p *OCRParser) upscaleImage(img image.Image, factor int) image.Image {
	bounds := img.Bounds()
	newWidth := bounds.Dx() * factor
	newHeight := bounds.Dy() * factor

	// Create new RGBA image
	upscaled := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// Nearest-neighbor upscaling
	for y := 0; y < newHeight; y++ {
		srcY := bounds.Min.Y + y/factor
		for x := 0; x < newWidth; x++ {
			srcX := bounds.Min.X + x/factor
			upscaled.Set(x, y, img.At(srcX, srcY))
		}
	}

	return upscaled
}

// cropBarcodeArea crops the barcode area from a Vergi Levhası image
// Based on the standard GIB (Gelir İdaresi Başkanlığı) PDF format,
// the barcode is located in the bottom-right corner, in the "ONAY KODU" section
func (p *OCRParser) cropBarcodeArea(img image.Image) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// The barcode is typically in the right 35% and bottom 25% of the document
	// These proportions are based on the standard Vergi Levhası layout
	x0 := bounds.Min.X + int(float64(width)*0.60)  // Start at 60% from left
	y0 := bounds.Min.Y + int(float64(height)*0.70) // Start at 70% from top
	x1 := bounds.Max.X - int(float64(width)*0.02)  // End at 98% width (small margin)
	y1 := bounds.Max.Y - int(float64(height)*0.02) // End at 98% height (small margin)

	return p.cropImage(img, image.Rect(x0, y0, x1, y1))
}

// getBarcodeCropRegions returns multiple potential barcode regions to try
func (p *OCRParser) getBarcodeCropRegions(bounds image.Rectangle) []image.Rectangle {
	width := bounds.Dx()
	height := bounds.Dy()

	return []image.Rectangle{
		// Bottom-right quadrant (most common location)
		image.Rect(
			bounds.Min.X+int(float64(width)*0.55),
			bounds.Min.Y+int(float64(height)*0.65),
			bounds.Max.X-int(float64(width)*0.01),
			bounds.Max.Y-int(float64(height)*0.01),
		),
		// Right third, bottom half
		image.Rect(
			bounds.Min.X+int(float64(width)*0.65),
			bounds.Min.Y+int(float64(height)*0.50),
			bounds.Max.X,
			bounds.Max.Y,
		),
		// Full right half
		image.Rect(
			bounds.Min.X+int(float64(width)*0.50),
			bounds.Min.Y,
			bounds.Max.X,
			bounds.Max.Y,
		),
		// Full bottom half
		image.Rect(
			bounds.Min.X,
			bounds.Min.Y+int(float64(height)*0.50),
			bounds.Max.X,
			bounds.Max.Y,
		),
	}
}

// cropImage crops a rectangular region from an image
func (p *OCRParser) cropImage(img image.Image, rect image.Rectangle) image.Image {
	// Ensure rect is within bounds
	bounds := img.Bounds()
	rect = rect.Intersect(bounds)
	if rect.Empty() {
		return nil
	}

	// Create a new RGBA image for the cropped region
	cropped := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(cropped, cropped.Bounds(), img, rect.Min, draw.Src)

	return cropped
}

// ExtractVKNFromPDFReaderWithImage extracts VKN from a PDF reader by extracting embedded images
// Uses pdfcpu for image extraction (pure Go, no external dependencies)
func (p *OCRParser) ExtractVKNFromPDFReaderWithImage(reader io.Reader) (string, error) {
	// Read all data first
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read PDF data: %w", err)
	}

	// Extract all embedded images using pdfcpu
	images, err := p.extractAllPDFImages(data)
	if err != nil {
		return "", fmt.Errorf("failed to extract images from PDF: %w", err)
	}

	if len(images) == 0 {
		return "", fmt.Errorf("no images found in PDF")
	}

	if p.debug {
		fmt.Printf("Found %d embedded images in PDF\n", len(images))
	}

	// Try each image for barcode scanning
	for i, img := range images {
		if p.debug {
			fmt.Printf("Scanning image %d: %dx%d\n", i+1, img.Bounds().Dx(), img.Bounds().Dy())
			_ = saveImage(img, fmt.Sprintf("debug_image_%d.png", i+1))
		}

		// Try Code128 barcode scan (VKN barcode is Code128)
		if vkn, err := p.scanCode128Barcode(img); err == nil && vkn != "" {
			if p.debug {
				fmt.Printf("Successfully extracted VKN from image %d: %s\n", i+1, vkn)
			}
			return vkn, nil
		}

		// Try general barcode scan
		if vkn, err := p.scanBarcode(img); err == nil && vkn != "" {
			if p.debug {
				fmt.Printf("Successfully extracted VKN from image %d: %s\n", i+1, vkn)
			}
			return vkn, nil
		}

		// Try upscaling if the image is small
		if img.Bounds().Dx() < 500 || img.Bounds().Dy() < 100 {
			upscaled := p.upscaleImage(img, 4)
			if p.debug {
				fmt.Printf("Upscaled image %d to: %dx%d\n", i+1, upscaled.Bounds().Dx(), upscaled.Bounds().Dy())
				_ = saveImage(upscaled, fmt.Sprintf("debug_image_%d_upscaled.png", i+1))
			}
			if vkn, err := p.scanCode128Barcode(upscaled); err == nil && vkn != "" {
				return vkn, nil
			}
			if vkn, err := p.scanBarcode(upscaled); err == nil && vkn != "" {
				return vkn, nil
			}
		}
	}

	return "", fmt.Errorf("could not extract VKN from PDF images")
}

// cropVKNTextArea crops the area where VKN number is typically printed
// In Vergi Levhası, VKN appears as text near the barcode in the top-right section
func (p *OCRParser) cropVKNTextArea(img image.Image) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// VKN text area: right side of document, upper portion where "VERGİ KİMLİK NO" section is
	// Based on the standard layout: right 40%, top 40%
	x0 := bounds.Min.X + int(float64(width)*0.55)
	y0 := bounds.Min.Y + int(float64(height)*0.10)
	x1 := bounds.Max.X - int(float64(width)*0.02)
	y1 := bounds.Min.Y + int(float64(height)*0.35)

	return p.cropImage(img, image.Rect(x0, y0, x1, y1))
}

// ExtractVKNFromPDFBytes extracts VKN from PDF bytes by extracting embedded images
// Uses pdfcpu for image extraction (pure Go, no external dependencies)
func (p *OCRParser) ExtractVKNFromPDFBytes(pdfData []byte) (string, error) {
	return p.ExtractVKNFromPDFReaderWithImage(bytes.NewReader(pdfData))
}

// looksLikeDate checks if a 10-digit string looks like a date pattern
func looksLikeDate(s string) bool {
	if len(s) != 10 {
		return false
	}
	// Check for DDMMYYYY patterns (17092025 style)
	// or YYYYMMDD patterns
	day := s[0:2]
	month := s[2:4]
	year := s[4:8]

	if isValidDay(day) && isValidMonth(month) && isValidYear(year) {
		return true
	}

	// Also check YYYYMMDD
	year = s[0:4]
	month = s[4:6]
	day = s[6:8]

	if isValidYear(year) && isValidMonth(month) && isValidDay(day) {
		return true
	}

	return false
}

func isValidDay(s string) bool {
	if len(s) != 2 {
		return false
	}
	d := int(s[0]-'0')*10 + int(s[1]-'0')
	return d >= 1 && d <= 31
}

func isValidMonth(s string) bool {
	if len(s) != 2 {
		return false
	}
	m := int(s[0]-'0')*10 + int(s[1]-'0')
	return m >= 1 && m <= 12
}

func isValidYear(s string) bool {
	if len(s) != 4 {
		return false
	}
	y := int(s[0]-'0')*1000 + int(s[1]-'0')*100 + int(s[2]-'0')*10 + int(s[3]-'0')
	return y >= 1900 && y <= 2100
}

// ExtractVKNFromImage extracts VKN from an image file
func (p *OCRParser) ExtractVKNFromImage(imagePath string) (string, error) {
	imgFile, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %w", err)
	}
	defer func(imgFile *os.File) {
		err := imgFile.Close()
		if err != nil {
			fmt.Printf("failed to close image file: %v\n", err)
		}
	}(imgFile)

	img, _, err := image.Decode(imgFile)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	return p.ExtractVKNFromImageData(img)
}

// ExtractVKNFromImageBytes extracts VKN from image bytes
func (p *OCRParser) ExtractVKNFromImageBytes(imgData []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	return p.ExtractVKNFromImageData(img)
}

// ExtractVKNFromImageData extracts VKN from an image.Image
func (p *OCRParser) ExtractVKNFromImageData(img image.Image) (string, error) {
	// Step 0: Try barcode scanning first (most reliable)
	if vkn, err := p.scanBarcode(img); err == nil && vkn != "" {
		if p.debug {
			fmt.Printf("Found VKN from barcode: %s\n", vkn)
		}
		return vkn, nil
	}

	// Step 1: Convert to grayscale
	grayImg := toGrayscale(img)

	if p.debug {
		err := saveImage(grayImg, "debug_01_grayscale.png")
		if err != nil {
			return "", err
		}
	}

	// Step 2: Binarize with adaptive threshold
	binaryImg := adaptiveBinarize(grayImg, 15, 10)

	if p.debug {
		err := saveImage(binaryImg, "debug_02_binary.png")
		if err != nil {
			return "", err
		}
	}

	// Step 3: Find connected components (potential digits)
	regions := findConnectedComponents(binaryImg)

	if p.debug {
		fmt.Printf("Found %d connected components\n", len(regions))
	}

	// Step 4: Filter regions that look like digits
	digitRegions := filterDigitRegions(regions, binaryImg.Bounds())

	if p.debug {
		fmt.Printf("Filtered to %d potential digits\n", len(digitRegions))
	}

	// Step 5: Group regions into lines and sort by x-coordinate
	sortedRegions := sortRegionsByPosition(digitRegions)

	// Step 6: Recognize each digit
	var allDigits strings.Builder
	for i, region := range sortedRegions {
		// Extract and normalize digit image
		digitImg := extractDigitImage(binaryImg, region)

		// Classify the digit
		digit, confidence := p.classifier.Classify(digitImg)

		if p.debug {
			fmt.Printf("Region %d at (%d,%d): digit=%d, confidence=%.2f\n",
				i, region.Min.X, region.Min.Y, digit, confidence)
			err := saveImage(digitImg, fmt.Sprintf("debug_digit_%02d.png", i))
			if err != nil {
				return "", err
			}
		}

		if confidence >= 0.3 {
			allDigits.WriteByte(byte('0' + digit))
		}
	}

	// Step 7: Find VKN pattern (10 consecutive digits starting with non-zero)
	digitStr := allDigits.String()
	if p.debug {
		fmt.Printf("All recognized digits: %s\n", digitStr)
	}

	re := regexp.MustCompile(`([1-9]\d{9})`)
	if match := re.FindString(digitStr); match != "" {
		return match, nil
	}

	// Try to find partial matches
	re2 := regexp.MustCompile(`(\d{10})`)
	if match := re2.FindString(digitStr); match != "" {
		return match, nil
	}

	return "", fmt.Errorf("no valid VKN found (recognized: %s)", digitStr)
}

// scanCode128Barcode attempts to decode a Code128 barcode specifically
// The VKN barcode in Turkish tax plates is a Code128 barcode
func (p *OCRParser) scanCode128Barcode(img image.Image) (string, error) {
	// Try scanning with different image orientations
	orientations := []int{0, 90, 180, 270}

	for _, rotation := range orientations {
		rotatedImg := img
		if rotation > 0 {
			rotatedImg = rotateImage(img, rotation)
		}

		// Try with original image
		if vkn, err := p.scanCode128Only(rotatedImg); err == nil && vkn != "" {
			return vkn, nil
		}

		// Try with enhanced contrast
		enhanced := p.enhanceBarcode(rotatedImg)
		if vkn, err := p.scanCode128Only(enhanced); err == nil && vkn != "" {
			return vkn, nil
		}
	}

	return "", fmt.Errorf("no Code128 barcode found")
}

// scanCode128Only scans image using only Code128 reader
func (p *OCRParser) scanCode128Only(img image.Image) (string, error) {
	// Convert image to BinaryBitmap for gozxing
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", fmt.Errorf("failed to create bitmap: %w", err)
	}

	// Use Code128 reader specifically
	reader := oned.NewCode128Reader()

	result, err := reader.Decode(bmp, nil)
	if err != nil {
		return "", fmt.Errorf("Code128 decode failed: %w", err)
	}

	text := result.GetText()
	if p.debug {
		fmt.Printf("Code128 decoded: %s\n", text)
	}

	// Extract VKN from the barcode text
	if vkn := p.extractVKNFromBarcodeText(text); vkn != "" {
		return vkn, nil
	}

	// If the text itself is a 10-digit number starting with non-zero, use it
	if len(text) == 10 && text[0] >= '1' && text[0] <= '9' {
		allDigits := true
		for _, ch := range text {
			if ch < '0' || ch > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return text, nil
		}
	}

	return "", fmt.Errorf("no VKN found in barcode text: %s", text)
}

// enhanceBarcode enhances the barcode image for better reading
func (p *OCRParser) enhanceBarcode(img image.Image) image.Image {
	bounds := img.Bounds()
	enhanced := image.NewGray(bounds)

	// Convert to high-contrast grayscale
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			gray := color.GrayModel.Convert(c).(color.Gray)

			// Apply threshold to make barcode more distinct
			if gray.Y > 128 {
				enhanced.SetGray(x, y, color.Gray{255})
			} else {
				enhanced.SetGray(x, y, color.Gray{0})
			}
		}
	}

	return enhanced
}

// scanBarcode attempts to decode a barcode from the image
func (p *OCRParser) scanBarcode(img image.Image) (string, error) {
	// Try scanning with different image orientations
	// Sometimes barcodes need to be rotated for proper detection
	orientations := []int{0, 90, 180, 270}

	for _, rotation := range orientations {
		rotatedImg := img
		if rotation > 0 {
			rotatedImg = rotateImage(img, rotation)
		}

		vkn, err := p.scanBarcodeOrientation(rotatedImg)
		if err == nil && vkn != "" {
			return vkn, nil
		}
	}

	return "", fmt.Errorf("no barcode found")
}

// scanBarcodeOrientation scans barcode in a specific orientation
func (p *OCRParser) scanBarcodeOrientation(img image.Image) (string, error) {
	// Convert image to BinaryBitmap for gozxing
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", fmt.Errorf("failed to create bitmap: %w", err)
	}

	// First try MultiFormatReader which tries all formats
	reader := oned.NewCode128Reader()

	result, err := reader.Decode(bmp, nil)
	if err == nil {
		text := result.GetText()
		if p.debug {
			fmt.Printf("Reader decoded: %s\n", text)
		}
		if vkn := p.extractVKNFromBarcodeText(text); vkn != "" {
			return vkn, nil
		}
	}

	// Try individual readers
	readers := []gozxing.Reader{
		oned.NewCode128Reader(),
		oned.NewCode39Reader(),
		oned.NewEAN13Reader(),
		oned.NewEAN8Reader(),
		oned.NewITFReader(),
		oned.NewCodaBarReader(),
		oned.NewUPCAReader(),
		oned.NewUPCEReader(),
	}

	var allDecodedTexts []string

	for _, reader := range readers {
		result, err := reader.Decode(bmp, nil)
		if err == nil {
			text := result.GetText()
			if p.debug {
				fmt.Printf("Barcode decoded with %T: %s\n", reader, text)
			}
			allDecodedTexts = append(allDecodedTexts, text)

			if vkn := p.extractVKNFromBarcodeText(text); vkn != "" {
				return vkn, nil
			}
		}
	}

	// If we decoded something but couldn't find VKN, try extracting digits
	for _, text := range allDecodedTexts {
		// Extract all digits from the decoded text
		var digits strings.Builder
		for _, ch := range text {
			if ch >= '0' && ch <= '9' {
				digits.WriteRune(ch)
			}
		}
		digitStr := digits.String()
		if len(digitStr) >= 10 {
			// Try to find VKN pattern
			re := regexp.MustCompile(`([1-9]\d{9})`)
			if match := re.FindString(digitStr); match != "" {
				return match, nil
			}
		}
	}

	return "", fmt.Errorf("no barcode found")
}

// extractVKNFromBarcodeText extracts VKN from barcode decoded text
func (p *OCRParser) extractVKNFromBarcodeText(text string) string {
	// Check if it's a valid VKN (10 digits starting with non-zero)
	re := regexp.MustCompile(`([1-9]\d{9})`)
	matches := re.FindAllString(text, -1)
	for _, match := range matches {
		if isValidVKN(match) {
			if p.debug {
				fmt.Printf("Valid VKN found in barcode: %s\n", match)
			}
			return match
		}
	}

	// If no valid VKN found via validation, still try to find 10-digit match
	if match := re.FindString(text); match != "" {
		return match
	}

	return ""
}

// rotateImage rotates an image by the specified degrees (90, 180, 270)
func rotateImage(img image.Image, degrees int) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	var rotated *image.RGBA
	switch degrees {
	case 90:
		rotated = image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				rotated.Set(h-1-y, x, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
	case 180:
		rotated = image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				rotated.Set(w-1-x, h-1-y, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
	case 270:
		rotated = image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				rotated.Set(y, w-1-x, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
	default:
		return img
	}
	return rotated
}

// isValidVKN validates a Turkish Tax Identification Number (Vergi Kimlik Numarası)
// This performs basic structural validation
func isValidVKN(vkn string) bool {
	if len(vkn) != 10 {
		return false
	}
	// VKN must start with a non-zero digit
	if vkn[0] == '0' {
		return false
	}

	// All characters must be digits
	for _, ch := range vkn {
		if ch < '0' || ch > '9' {
			return false
		}
	}

	// VKN should not look like a date (DDMMYYYYXX or YYYYMMDDXX patterns)
	if looksLikeDate(vkn) {
		return false
	}

	return true
}

// ============================================================================
// Digit Classifier - Pure Go Implementation
// ============================================================================

// DigitClassifier recognizes digits using feature extraction
type DigitClassifier struct {
	// Pre-computed feature weights for each digit (0-9)
	weights [10]DigitFeatureWeights
}

// DigitFeatureWeights contains weights for matching a specific digit
type DigitFeatureWeights struct {
	horizontalSymmetry float64
	verticalSymmetry   float64
	topHeavy           float64
	bottomHeavy        float64
	leftHeavy          float64
	rightHeavy         float64
	centerDensity      float64
	aspectRatio        float64
	holeCount          float64
	crossings          float64
}

// NewDigitClassifier creates a classifier with pre-trained weights
func NewDigitClassifier() *DigitClassifier {
	c := &DigitClassifier{}

	// These weights are derived from MNIST digit characteristics
	// Each digit has distinctive features

	// 0: Round, symmetric, hole in center, wide
	c.weights[0] = DigitFeatureWeights{
		horizontalSymmetry: 0.8, verticalSymmetry: 0.7,
		topHeavy: 0.5, bottomHeavy: 0.5,
		leftHeavy: 0.5, rightHeavy: 0.5,
		centerDensity: 0.3, aspectRatio: 0.7,
		holeCount: 1.0, crossings: 0.4,
	}

	// 1: Narrow, tall, mostly in center/right, no holes
	c.weights[1] = DigitFeatureWeights{
		horizontalSymmetry: 0.6, verticalSymmetry: 0.5,
		topHeavy: 0.5, bottomHeavy: 0.5,
		leftHeavy: 0.3, rightHeavy: 0.6,
		centerDensity: 0.7, aspectRatio: 0.3,
		holeCount: 0.0, crossings: 0.2,
	}

	// 2: Top curve, diagonal, bottom horizontal
	c.weights[2] = DigitFeatureWeights{
		horizontalSymmetry: 0.4, verticalSymmetry: 0.3,
		topHeavy: 0.6, bottomHeavy: 0.5,
		leftHeavy: 0.4, rightHeavy: 0.5,
		centerDensity: 0.4, aspectRatio: 0.6,
		holeCount: 0.0, crossings: 0.5,
	}

	// 3: Right side heavy, two bumps
	c.weights[3] = DigitFeatureWeights{
		horizontalSymmetry: 0.3, verticalSymmetry: 0.5,
		topHeavy: 0.5, bottomHeavy: 0.5,
		leftHeavy: 0.3, rightHeavy: 0.7,
		centerDensity: 0.4, aspectRatio: 0.6,
		holeCount: 0.0, crossings: 0.6,
	}

	// 4: Vertical line on right, horizontal in middle
	c.weights[4] = DigitFeatureWeights{
		horizontalSymmetry: 0.4, verticalSymmetry: 0.4,
		topHeavy: 0.6, bottomHeavy: 0.4,
		leftHeavy: 0.4, rightHeavy: 0.6,
		centerDensity: 0.5, aspectRatio: 0.6,
		holeCount: 0.0, crossings: 0.5,
	}

	// 5: Top horizontal, middle, bottom curve
	c.weights[5] = DigitFeatureWeights{
		horizontalSymmetry: 0.4, verticalSymmetry: 0.4,
		topHeavy: 0.55, bottomHeavy: 0.45,
		leftHeavy: 0.5, rightHeavy: 0.5,
		centerDensity: 0.45, aspectRatio: 0.6,
		holeCount: 0.0, crossings: 0.5,
	}

	// 6: Top curve/tail, bottom loop with hole
	c.weights[6] = DigitFeatureWeights{
		horizontalSymmetry: 0.5, verticalSymmetry: 0.4,
		topHeavy: 0.4, bottomHeavy: 0.6,
		leftHeavy: 0.55, rightHeavy: 0.45,
		centerDensity: 0.5, aspectRatio: 0.6,
		holeCount: 0.8, crossings: 0.5,
	}

	// 7: Top horizontal, diagonal down
	c.weights[7] = DigitFeatureWeights{
		horizontalSymmetry: 0.4, verticalSymmetry: 0.3,
		topHeavy: 0.7, bottomHeavy: 0.3,
		leftHeavy: 0.4, rightHeavy: 0.6,
		centerDensity: 0.35, aspectRatio: 0.6,
		holeCount: 0.0, crossings: 0.3,
	}

	// 8: Two stacked loops, very symmetric
	c.weights[8] = DigitFeatureWeights{
		horizontalSymmetry: 0.85, verticalSymmetry: 0.7,
		topHeavy: 0.5, bottomHeavy: 0.5,
		leftHeavy: 0.5, rightHeavy: 0.5,
		centerDensity: 0.4, aspectRatio: 0.65,
		holeCount: 1.0, crossings: 0.6,
	}

	// 9: Top loop with hole, bottom tail
	c.weights[9] = DigitFeatureWeights{
		horizontalSymmetry: 0.5, verticalSymmetry: 0.4,
		topHeavy: 0.6, bottomHeavy: 0.4,
		leftHeavy: 0.45, rightHeavy: 0.55,
		centerDensity: 0.5, aspectRatio: 0.6,
		holeCount: 0.8, crossings: 0.5,
	}

	return c
}

// Classify returns the most likely digit and confidence
func (c *DigitClassifier) Classify(img *image.Gray) (int, float64) {
	features := extractFeatures(img)

	bestDigit := 0
	bestScore := -1.0

	for digit := 0; digit < 10; digit++ {
		score := c.matchScore(features, c.weights[digit])
		if score > bestScore {
			bestScore = score
			bestDigit = digit
		}
	}

	// Normalize confidence to 0-1 range
	confidence := bestScore
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0 {
		confidence = 0
	}

	return bestDigit, confidence
}

func (c *DigitClassifier) matchScore(f DigitFeatures, w DigitFeatureWeights) float64 {
	score := 0.0

	// Compare each feature (higher score = closer match)
	score += 1.0 - math.Abs(f.horizontalSymmetry-w.horizontalSymmetry)
	score += 1.0 - math.Abs(f.verticalSymmetry-w.verticalSymmetry)
	score += 1.0 - math.Abs(f.topHeavy-w.topHeavy)
	score += 1.0 - math.Abs(f.bottomHeavy-w.bottomHeavy)
	score += 1.0 - math.Abs(f.leftHeavy-w.leftHeavy)
	score += 1.0 - math.Abs(f.rightHeavy-w.rightHeavy)
	score += 1.0 - math.Abs(f.centerDensity-w.centerDensity)
	score += (1.0 - math.Abs(f.aspectRatio-w.aspectRatio)) * 0.5
	score += (1.0 - math.Abs(f.holeCount-w.holeCount)) * 1.5 // Holes are very discriminative
	score += 1.0 - math.Abs(f.crossings-w.crossings)

	return score / 10.0 // Normalize
}

// DigitFeatures contains extracted features from a digit image
type DigitFeatures struct {
	horizontalSymmetry float64
	verticalSymmetry   float64
	topHeavy           float64
	bottomHeavy        float64
	leftHeavy          float64
	rightHeavy         float64
	centerDensity      float64
	aspectRatio        float64
	holeCount          float64
	crossings          float64
}

func extractFeatures(img *image.Gray) DigitFeatures {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	var f DigitFeatures
	totalMass := 0.0
	topMass, bottomMass := 0.0, 0.0
	leftMass, rightMass := 0.0, 0.0
	centerMass := 0.0

	midY := height / 2
	midX := width / 2
	centerStartX, centerEndX := width/4, 3*width/4
	centerStartY, centerEndY := height/4, 3*height/4

	// Calculate mass distribution
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Invert: black (foreground) = high value
			val := float64(255-img.GrayAt(bounds.Min.X+x, bounds.Min.Y+y).Y) / 255.0
			totalMass += val

			if y < midY {
				topMass += val
			} else {
				bottomMass += val
			}

			if x < midX {
				leftMass += val
			} else {
				rightMass += val
			}

			if x >= centerStartX && x < centerEndX && y >= centerStartY && y < centerEndY {
				centerMass += val
			}
		}
	}

	if totalMass > 0 {
		f.topHeavy = topMass / totalMass
		f.bottomHeavy = bottomMass / totalMass
		f.leftHeavy = leftMass / totalMass
		f.rightHeavy = rightMass / totalMass
		centerArea := float64((centerEndX - centerStartX) * (centerEndY - centerStartY))
		f.centerDensity = centerMass / (totalMass * centerArea / float64(width*height))
		if f.centerDensity > 1 {
			f.centerDensity = 1
		}
	}

	// Symmetry
	f.horizontalSymmetry = calculateHorizontalSymmetry(img)
	f.verticalSymmetry = calculateVerticalSymmetry(img)

	// Aspect ratio
	f.aspectRatio = float64(width) / float64(height)
	if f.aspectRatio > 1 {
		f.aspectRatio = 1 / f.aspectRatio
	}

	// Holes (approximate by counting enclosed regions)
	f.holeCount = float64(countHoles(img)) / 2.0
	if f.holeCount > 1 {
		f.holeCount = 1
	}

	// Horizontal crossings (how many times we cross black when scanning horizontally)
	f.crossings = calculateCrossings(img)

	return f
}

func calculateHorizontalSymmetry(img *image.Gray) float64 {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	totalDiff := 0.0
	count := 0

	for y := 0; y < height; y++ {
		for x := 0; x < width/2; x++ {
			left := float64(img.GrayAt(bounds.Min.X+x, bounds.Min.Y+y).Y)
			right := float64(img.GrayAt(bounds.Min.X+width-1-x, bounds.Min.Y+y).Y)
			totalDiff += math.Abs(left-right) / 255.0
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return 1.0 - totalDiff/float64(count)
}

func calculateVerticalSymmetry(img *image.Gray) float64 {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	totalDiff := 0.0
	count := 0

	for y := 0; y < height/2; y++ {
		for x := 0; x < width; x++ {
			top := float64(img.GrayAt(bounds.Min.X+x, bounds.Min.Y+y).Y)
			bottom := float64(img.GrayAt(bounds.Min.X+x, bounds.Min.Y+height-1-y).Y)
			totalDiff += math.Abs(top-bottom) / 255.0
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return 1.0 - totalDiff/float64(count)
}

func countHoles(img *image.Gray) int {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Create a copy and flood fill from edges
	visited := make([][]bool, height)
	for i := range visited {
		visited[i] = make([]bool, width)
	}

	// Flood fill from edges (mark background)
	var floodFill func(x, y int)
	floodFill = func(x, y int) {
		if x < 0 || x >= width || y < 0 || y >= height {
			return
		}
		if visited[y][x] {
			return
		}
		// White pixel (background) or edge
		if img.GrayAt(bounds.Min.X+x, bounds.Min.Y+y).Y > 128 {
			visited[y][x] = true
			floodFill(x+1, y)
			floodFill(x-1, y)
			floodFill(x, y+1)
			floodFill(x, y-1)
		}
	}

	// Fill from all edges
	for x := 0; x < width; x++ {
		floodFill(x, 0)
		floodFill(x, height-1)
	}
	for y := 0; y < height; y++ {
		floodFill(0, y)
		floodFill(width-1, y)
	}

	// Count remaining unvisited white regions (holes)
	holes := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if !visited[y][x] && img.GrayAt(bounds.Min.X+x, bounds.Min.Y+y).Y > 128 {
				holes++
				// Flood fill this hole to avoid counting it multiple times
				var fillHole func(hx, hy int)
				fillHole = func(hx, hy int) {
					if hx < 0 || hx >= width || hy < 0 || hy >= height {
						return
					}
					if visited[hy][hx] {
						return
					}
					if img.GrayAt(bounds.Min.X+hx, bounds.Min.Y+hy).Y > 128 {
						visited[hy][hx] = true
						fillHole(hx+1, hy)
						fillHole(hx-1, hy)
						fillHole(hx, hy+1)
						fillHole(hx, hy-1)
					}
				}
				fillHole(x, y)
			}
		}
	}

	return holes
}

func calculateCrossings(img *image.Gray) float64 {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	totalCrossings := 0
	lines := 0

	// Sample horizontal lines
	for y := height / 4; y < 3*height/4; y += height / 8 {
		if y >= height {
			continue
		}
		crossings := 0
		inForeground := false
		for x := 0; x < width; x++ {
			isForeground := img.GrayAt(bounds.Min.X+x, bounds.Min.Y+y).Y < 128
			if isForeground != inForeground {
				crossings++
				inForeground = isForeground
			}
		}
		totalCrossings += crossings
		lines++
	}

	if lines == 0 {
		return 0
	}

	avg := float64(totalCrossings) / float64(lines)
	// Normalize: 2 crossings (one stroke) = 0.2, 4 crossings = 0.4, etc.
	return math.Min(avg/10.0, 1.0)
}

// ============================================================================
// Image Processing Functions
// ============================================================================

func toGrayscale(img image.Image) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			gray.Set(x, y, color.GrayModel.Convert(c).(color.Gray))
		}
	}
	return gray
}

func adaptiveBinarize(img *image.Gray, blockSize, c int) *image.Gray {
	bounds := img.Bounds()
	binary := image.NewGray(bounds)
	halfBlock := blockSize / 2

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			sum := 0
			count := 0
			for dy := -halfBlock; dy <= halfBlock; dy++ {
				for dx := -halfBlock; dx <= halfBlock; dx++ {
					ny, nx := y+dy, x+dx
					if ny >= bounds.Min.Y && ny < bounds.Max.Y && nx >= bounds.Min.X && nx < bounds.Max.X {
						sum += int(img.GrayAt(nx, ny).Y)
						count++
					}
				}
			}

			threshold := sum/count - c
			if int(img.GrayAt(x, y).Y) < threshold {
				binary.SetGray(x, y, color.Gray{0}) // Black (foreground)
			} else {
				binary.SetGray(x, y, color.Gray{255}) // White (background)
			}
		}
	}

	return binary
}

func findConnectedComponents(img *image.Gray) []image.Rectangle {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	visited := make([][]bool, height)
	for i := range visited {
		visited[i] = make([]bool, width)
	}

	var regions []image.Rectangle

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if visited[y][x] {
				continue
			}
			if img.GrayAt(bounds.Min.X+x, bounds.Min.Y+y).Y == 0 { // Black pixel
				region := floodFillRegion(img, visited, x, y, bounds)
				if region.Dx() >= 3 && region.Dy() >= 5 {
					regions = append(regions, region)
				}
			}
			visited[y][x] = true
		}
	}

	return regions
}

func floodFillRegion(img *image.Gray, visited [][]bool, startX, startY int, bounds image.Rectangle) image.Rectangle {
	width, height := bounds.Dx(), bounds.Dy()
	minX, minY := startX, startY
	maxX, maxY := startX, startY

	type point struct{ x, y int }
	stack := []point{{startX, startY}}

	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if p.x < 0 || p.x >= width || p.y < 0 || p.y >= height {
			continue
		}
		if visited[p.y][p.x] {
			continue
		}
		if img.GrayAt(bounds.Min.X+p.x, bounds.Min.Y+p.y).Y != 0 {
			continue
		}

		visited[p.y][p.x] = true

		if p.x < minX {
			minX = p.x
		}
		if p.x > maxX {
			maxX = p.x
		}
		if p.y < minY {
			minY = p.y
		}
		if p.y > maxY {
			maxY = p.y
		}

		stack = append(stack, point{p.x + 1, p.y})
		stack = append(stack, point{p.x - 1, p.y})
		stack = append(stack, point{p.x, p.y + 1})
		stack = append(stack, point{p.x, p.y - 1})
	}

	return image.Rect(bounds.Min.X+minX, bounds.Min.Y+minY, bounds.Min.X+maxX+1, bounds.Min.Y+maxY+1)
}

func filterDigitRegions(regions []image.Rectangle, imgBounds image.Rectangle) []image.Rectangle {
	var filtered []image.Rectangle
	imgHeight := imgBounds.Dy()
	imgWidth := imgBounds.Dx()

	for _, r := range regions {
		w, h := r.Dx(), r.Dy()
		aspectRatio := float64(w) / float64(h)

		// Digits typically have aspect ratio between 0.2 and 1.2
		if aspectRatio < 0.15 || aspectRatio > 1.5 {
			continue
		}

		// Not too small
		if w < 5 || h < 8 {
			continue
		}

		// Not too large (more than 1/3 of image)
		if w > imgWidth/3 || h > imgHeight/2 {
			continue
		}

		filtered = append(filtered, r)
	}

	return filtered
}

func sortRegionsByPosition(regions []image.Rectangle) []image.Rectangle {
	// Group by approximate y-coordinate (same row)
	sort.Slice(regions, func(i, j int) bool {
		// If on same row (y within 50% of height), sort by x
		ri, rj := regions[i], regions[j]
		rowThreshold := (ri.Dy() + rj.Dy()) / 4

		if abs(ri.Min.Y-rj.Min.Y) < rowThreshold {
			return ri.Min.X < rj.Min.X
		}
		return ri.Min.Y < rj.Min.Y
	})

	return regions
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func extractDigitImage(img *image.Gray, region image.Rectangle) *image.Gray {
	// Add padding
	padding := 4
	w := region.Dx() + 2*padding
	h := region.Dy() + 2*padding

	// Make it square (helps with recognition)
	size := w
	if h > size {
		size = h
	}

	result := image.NewGray(image.Rect(0, 0, size, size))

	// Fill with white
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			result.SetGray(x, y, color.Gray{255})
		}
	}

	// Center the digit
	offsetX := (size - region.Dx()) / 2
	offsetY := (size - region.Dy()) / 2

	// Copy digit
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			result.SetGray(x-region.Min.X+offsetX, y-region.Min.Y+offsetY, img.GrayAt(x, y))
		}
	}

	return result
}

func saveImage(img image.Image, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			fmt.Printf("Failed to close file %s: %v\n", filename, err)
		}
	}(f)
	return png.Encode(f, img)
}

// ============================================================================
// Helper exports
// ============================================================================

// SaveDebugImage saves an image for debugging
func (p *OCRParser) SaveDebugImage(img image.Image, filename string) error {
	return saveImage(img, filename)
}
