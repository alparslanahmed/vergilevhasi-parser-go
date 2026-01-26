package vergilevhasi

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

// Parser is responsible for parsing Turkish tax plate PDFs
type Parser struct {
	// Options for parsing
	debug bool
}

// NewParser creates a new Parser instance
func NewParser() *Parser {
	return &Parser{
		debug: false,
	}
}

// SetDebug enables or disables debug mode
func (p *Parser) SetDebug(debug bool) {
	p.debug = debug
}

// ParseFile parses a tax plate PDF file and returns structured data
func (p *Parser) ParseFile(filepath string) (*VergiLevhasi, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return p.Parse(file)
}

// Parse parses a tax plate PDF from an io.ReadSeeker and returns structured data
func (p *Parser) Parse(reader io.ReadSeeker) (*VergiLevhasi, error) {
	// Read all content into a buffer
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF data: %w", err)
	}

	// Create a reader from the data
	rs := bytes.NewReader(data)

	// Create PDF reader using ledongthuc/pdf
	pdfReader, err := pdf.NewReader(rs, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to create PDF reader: %w", err)
	}

	// Extract text from all pages using GetPlainText
	textReader, err := pdfReader.GetPlainText()
	if err != nil {
		return nil, fmt.Errorf("failed to extract text from PDF: %w", err)
	}

	textBytes, err := io.ReadAll(textReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read extracted text: %w", err)
	}

	rawText := string(textBytes)

	// Also try to extract text row by row from each page to capture more content
	var additionalText strings.Builder
	numPages := pdfReader.NumPage()
	for pageNum := 1; pageNum <= numPages; pageNum++ {
		page := pdfReader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		// Try GetTextByRow
		rows, err := page.GetTextByRow()
		if err == nil {
			for _, row := range rows {
				for _, word := range row.Content {
					additionalText.WriteString(word.S)
					additionalText.WriteString(" ")
				}
				additionalText.WriteString("\n")
			}
		}

		// Also try Content() which might have more raw text
		content := page.Content()
		for _, text := range content.Text {
			additionalText.WriteString(text.S)
			additionalText.WriteString(" ")
		}
	}

	// Try to extract VKN from raw PDF data (sometimes it's encoded differently)
	vknFromRaw := p.extractVKNFromRawPDF(data)

	// Combine all extraction methods
	combinedText := rawText + "\n" + additionalText.String()
	if vknFromRaw != "" {
		combinedText += "\nVergi Kimlik No: " + vknFromRaw
	}

	if p.debug {
		fmt.Println("Extracted Text:")
		fmt.Println(combinedText)
	}

	// Parse the extracted text
	vergiLevhasi := &VergiLevhasi{
		RawText: combinedText,
	}

	p.parseContent(vergiLevhasi, combinedText)

	return vergiLevhasi, nil
}

// extractVKNFromRawPDF searches for 10-digit VKN patterns in raw PDF data
func (p *Parser) extractVKNFromRawPDF(data []byte) string {
	// Convert to string for pattern matching
	rawStr := string(data)

	// Look for 10-digit numbers that could be VKN
	// VKN pattern: 10 consecutive digits, often preceded by markers
	vknRe := regexp.MustCompile(`\((\d{10})\)`)
	matches := vknRe.FindAllStringSubmatch(rawStr, -1)
	for _, match := range matches {
		if len(match) > 1 {
			vkn := match[1]
			// Validate: VKN should start with a non-zero digit
			if vkn[0] != '0' {
				return vkn
			}
		}
	}

	// Also try without parentheses
	vknRe2 := regexp.MustCompile(`\b(\d{10})\b`)
	matches2 := vknRe2.FindAllStringSubmatch(rawStr, -1)
	for _, match := range matches2 {
		if len(match) > 1 {
			vkn := match[1]
			// Validate: VKN should start with a non-zero digit
			if vkn[0] != '0' {
				return vkn
			}
		}
	}

	return ""
}

// parseContent extracts structured data from the raw text
func (p *Parser) parseContent(vl *VergiLevhasi, text string) {
	// Parse using position-based extraction for the GIB PDF format
	lines := strings.Split(text, "\n")

	// Helper function to check if line contains any of the keywords (case-insensitive for Turkish)
	containsAny := func(line string, keywords ...string) bool {
		lineLower := strings.ToLower(line)
		for _, kw := range keywords {
			if strings.Contains(lineLower, strings.ToLower(kw)) {
				return true
			}
		}
		return false
	}

	// Try traditional format first (with colons)
	// Extract Adı Soyadı (Full Name) - traditional format with colon
	vl.AdiSoyadi = p.extractField(text, []string{
		`(?i)adı\s*soyadı\s*[:：]\s*(.+?)(?:\n|$)`,
		`(?i)ad[ıi]\s*soyad[ıi]\s*[:：]\s*(.+?)(?:\n|$)`,
	})

	// Extract Ticaret Ünvanı - traditional format
	vl.TicaretUnvani = p.extractField(text, []string{
		`(?i)ticaret\s*ünvanı\s*[:：]\s*(.+?)(?:\n|$)`,
		`(?i)ticaret\s+ünvan[ıi]\s*[:：]\s*(.+?)(?:\n|$)`,
	})

	// Extract İş Yeri Adresi - traditional format
	vl.IsYeriAdresi = p.extractField(text, []string{
		`(?i)iş\s*yeri\s*adresi\s*[:：]\s*(.+?)(?:\n|$)`,
		`(?i)[iİ]ş\s*[yY]eri\s*[aA]dresi\s*[:：]\s*(.+?)(?:\n|$)`,
	})

	// Extract Vergi Dairesi - traditional format
	vl.VergiDairesi = p.extractField(text, []string{
		`(?i)vergi\s*dairesi\s*[:：]\s*(.+?)(?:\n|$)`,
	})

	// Extract Vergi Kimlik No - traditional format
	vl.VergiKimlikNo = p.extractField(text, []string{
		`(?i)vergi\s*kimlik\s*no\s*[:：]\s*(\d{10})`,
		`(?i)v\.?k\.?n\.?\s*[:：]\s*(\d{10})`,
	})

	// Extract TC Kimlik No - traditional format
	vl.TCKimlikNo = p.extractField(text, []string{
		`(?i)t\.?c\.?\s*kimlik\s*no\s*[:：]\s*(\d{11})`,
		`(?i)tckn\s*[:：]\s*(\d{11})`,
	})

	// Extract İşe Başlama Tarihi - traditional format
	dateStr := p.extractField(text, []string{
		`(?i)işe\s*başlama\s*tarihi\s*[:：]\s*(\d{2}[./-]\d{2}[./-]\d{4})`,
		`(?i)[iİ]şe\s*[bB]aşlama\s*[tT]arihi\s*[:：]\s*(\d{2}[./-]\d{2}[./-]\d{4})`,
	})
	if dateStr != "" {
		if date, err := p.parseDate(dateStr); err == nil {
			vl.IseBaslamaTarihi = &date
		}
	}

	// If traditional format didn't work, try GIB PDF format (without colons)

	// Extract Ticaret Ünvanı - GIB format: look for lines containing company suffixes
	if vl.TicaretUnvani == "" {
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if len(trimmedLine) < 15 {
				continue
			}
			// Check for company name indicators
			if containsAny(line, "ŞİRKET", "SIRKET", "LİMİTED", "LIMITED", "A.Ş", "A.S.") {
				// Make sure it's not just a label
				if !containsAny(line, "ÜNVAN", "NVANI", "UNVANI") &&
					!containsAny(line, "VERGİ TR", "VERGI TR", "VERGİDAİ", "VERGIDAI") {
					vl.TicaretUnvani = trimmedLine
					break
				}
			}
		}
	}

	// If we still didn't find it, look for lines with SANAYİ VE TİCARET pattern
	if vl.TicaretUnvani == "" {
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if len(trimmedLine) < 15 {
				continue
			}
			if containsAny(line, "SANAYİ", "SANAYI") && containsAny(line, "TİCARET", "TICARET") {
				if !containsAny(line, "ÜNVAN", "NVANI", "UNVANI") {
					vl.TicaretUnvani = trimmedLine
					break
				}
			}
		}
	}

	// Extract İş Yeri Adresi - GIB format: look for address patterns
	if vl.IsYeriAdresi == "" {
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			// Address usually contains street/district markers (with proper suffixes)
			// Be more specific to avoid matching "TC Kimlik No:" type lines
			hasAddressMarker := containsAny(line, "MAH.", "MAH ", "CAD.", "CAD ", "SOK.", "SOK ", "KAPI NO")
			// Also check for "NO:" but only if preceded by address-related words
			if !hasAddressMarker && strings.Contains(strings.ToUpper(line), "NO:") {
				// Check if this looks like an address (has building/apartment indicators)
				if containsAny(line, "KAPI", "DAİRE", "DAIRE", "KAT", "BLOK", "TOWER", "PLAZA") {
					hasAddressMarker = true
				}
			}

			if hasAddressMarker {
				// Exclude activity code lines and ID number lines
				if !containsAny(line, "FAALİYET", "FAALIYET", "KİMLİK", "KIMLIK") {
					if vl.IsYeriAdresi == "" {
						vl.IsYeriAdresi = trimmedLine
						// Only take the first address line to avoid duplicates
						break
					}
				}
			}
		}
	}

	// Extract Vergi Dairesi - GIB format: look for known tax office names
	if vl.VergiDairesi == "" {
		knownTaxOffices := []string{
			"KAĞITHANE", "KAGITHANE", "ŞİŞLİ", "SISLI", "KADIKÖY", "KADIKOY",
			"ÜSKÜDAR", "USKUDAR", "BEŞİKTAŞ", "BESIKTAS", "BEYOĞLU", "BEYOGLU",
			"BAKIRKÖY", "BAKIRKOY", "FATİH", "FATIH", "MALTEPE", "KARTAL",
			"ANKARA", "İZMİR", "IZMIR", "BURSA", "ANTALYA", "KONYA",
			"ATAŞEHİR", "ATASEHIR", "PENDİK", "PENDIK", "TUZLA", "SULTANBEYLİ",
			"SANCAKTEPE", "ÜMRANİYE", "UMRANIYE", "ÇEKMEKÖY", "CEKMEKOY",
		}

		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			upperLine := strings.ToUpper(trimmedLine)
			// Check if the line is exactly a tax office name (standalone line)
			for _, office := range knownTaxOffices {
				if upperLine == strings.ToUpper(office) {
					vl.VergiDairesi = trimmedLine
					break
				}
			}
			if vl.VergiDairesi != "" {
				break
			}
		}
	}

	// Extract Vergi Kimlik No - GIB format: look for 10-digit tax ID
	if vl.VergiKimlikNo == "" {
		vl.VergiKimlikNo = p.extractField(text, []string{
			`(?m)^(\d{10})$`,
			`\b(\d{10})\b`,
		})
	}

	// Extract TC Kimlik No - GIB format: look for 11-digit Turkish ID
	if vl.TCKimlikNo == "" {
		vl.TCKimlikNo = p.extractField(text, []string{
			`(?m)^(\d{11})$`,
			`\b(\d{11})\b`,
		})
	}

	// Extract İşe Başlama Tarihi - GIB format: look for date patterns (DD.MM.YYYY)
	if vl.IseBaslamaTarihi == nil {
		dateRe := regexp.MustCompile(`(\d{2}\.\d{2}\.\d{4})`)
		dateMatches := dateRe.FindAllString(text, -1)
		if len(dateMatches) > 0 {
			// Use the first date found (usually the İşe Başlama Tarihi)
			if date, err := p.parseDate(dateMatches[0]); err == nil {
				vl.IseBaslamaTarihi = &date
			}
		}
	}

	// For companies, don't use AdiSoyadi from label extraction
	// AdiSoyadi is only for individual taxpayers
	if vl.TicaretUnvani != "" && vl.AdiSoyadi != "" {
		// If we have a company name, check if AdiSoyadi is actually a label (not a real name)
		adiSoyadiUpper := strings.ToUpper(vl.AdiSoyadi)
		if containsAny(adiSoyadiUpper, "TİCARET", "TICARET", "NVANI", "ÜNVANI", "UNVANI") {
			vl.AdiSoyadi = ""
		}
	}

	// If no company name and no individual name from traditional format,
	// try to extract individual name from GIB format
	if vl.TicaretUnvani == "" && vl.AdiSoyadi == "" {
		// Try to find name after "ADI SOYADI" label
		for i, line := range lines {
			if containsAny(line, "ADI SOYADI", "ADISOYADI", "ADI SOYADΙ") {
				// Check the next few lines for a name
				for j := i + 1; j < len(lines) && j < i+3; j++ {
					nextLine := strings.TrimSpace(lines[j])
					if len(nextLine) > 3 &&
						!containsAny(nextLine, "TİCARET", "TICARET", "VERGİ", "VERGI", "İŞ YERİ", "IS YERI") {
						vl.AdiSoyadi = nextLine
						break
					}
				}
				break
			}
		}
	}

	// Extract Vergi Türü (Tax Types)
	vl.VergiTuru = p.extractTaxTypes(text)

	// Extract Faaliyet Kodları (Activity Codes)
	vl.FaaliyetKodlari = p.extractActivities(text)

	// Extract Geçmiş Matrahlar (Historical Tax Bases)
	vl.GecmisMatra = p.extractTaxBases(text)

	// Handle "Yeni işe başlama" (new business) case
	// In this case, there's no matrah data - the year shown is the registration year
	if containsAny(text, "Yeni işe başlama", "Yeni ise baslama") {
		// Clear matrah data that might have been incorrectly parsed
		// (e.g., activity code numbers being mistaken for amounts)
		var validMatrahlar []Matrah
		for _, m := range vl.GecmisMatra {
			// Skip if the amount looks like it's from activity code (e.g., 621 from 621000)
			// or if it's too small to be a real matrah
			if m.Tutar > 1000 {
				validMatrahlar = append(validMatrahlar, m)
			}
		}
		vl.GecmisMatra = validMatrahlar
	}
}

// extractField extracts a field using multiple regex patterns
func (p *Parser) extractField(text string, patterns []string) string {
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}
	return ""
}

// parseDate parses a date string in Turkish format
func (p *Parser) parseDate(dateStr string) (time.Time, error) {
	// Try multiple date formats
	formats := []string{
		"02.01.2006",
		"02/01/2006",
		"02-01-2006",
		"2.1.2006",
		"2/1/2006",
	}

	for _, format := range formats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return date, nil
		}
	}

	return time.Time{}, fmt.Errorf("failed to parse date: %s", dateStr)
}

// extractTaxTypes extracts tax types from the text
func (p *Parser) extractTaxTypes(text string) []string {
	var types []string

	// Common tax types in Turkish tax system
	taxTypes := []string{
		"Gelir Vergisi",
		"Kurumlar Vergisi",
		"Katma Değer Vergisi",
		"KDV",
		"Muhtasar",
		"Geçici Vergi",
		"Damga Vergisi",
		"Bağ-Kur",
		"SGK",
	}

	textLower := strings.ToLower(text)
	for _, taxType := range taxTypes {
		if strings.Contains(textLower, strings.ToLower(taxType)) {
			types = append(types, taxType)
		}
	}

	return types
}

// extractActivities extracts activity codes and names
func (p *Parser) extractActivities(text string) []Faaliyet {
	var activities []Faaliyet
	seen := make(map[string]bool)

	// Pattern for activity codes (usually 4-6 digits followed by description)
	re := regexp.MustCompile(`(?m)(\d{4,6})\s*[-–]\s*(.{10,100})`)
	matches := re.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) > 2 {
			kod := strings.TrimSpace(match[1])
			if !seen[kod] {
				seen[kod] = true
				activities = append(activities, Faaliyet{
					Kod: kod,
					Ad:  strings.TrimSpace(match[2]),
				})
			}
		}
	}

	return activities
}

// extractTaxBases extracts historical tax base information
func (p *Parser) extractTaxBases(text string) []Matrah {
	var matrahlar []Matrah

	// Pattern for year and amount
	re := regexp.MustCompile(`(?m)(\d{4})\s*(?:yılı)?\s*.*?(\d{1,3}(?:[.,]\d{3})*(?:[.,]\d{2})?)\s*(?:TL|₺)?`)
	matches := re.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) > 2 {
			year, err := strconv.Atoi(match[1])
			if err != nil || year < 1900 || year > 2100 {
				continue
			}

			// Parse amount
			amountStr := strings.ReplaceAll(match[2], ".", "")
			amountStr = strings.ReplaceAll(amountStr, ",", ".")
			amount, err := strconv.ParseFloat(amountStr, 64)
			if err != nil {
				continue
			}

			matrahlar = append(matrahlar, Matrah{
				Yil:   year,
				Tutar: amount,
			})
		}
	}

	return matrahlar
}
