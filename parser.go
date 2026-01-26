package vergilevhasi

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
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
	// Open the PDF
	pdfReader, err := model.NewPdfReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create PDF reader: %w", err)
	}

	// Extract text from all pages
	var fullText strings.Builder
	numPages, err := pdfReader.GetNumPages()
	if err != nil {
		return nil, fmt.Errorf("failed to get number of pages: %w", err)
	}

	for i := 1; i <= numPages; i++ {
		page, err := pdfReader.GetPage(i)
		if err != nil {
			return nil, fmt.Errorf("failed to get page %d: %w", i, err)
		}

		ex, err := extractor.New(page)
		if err != nil {
			return nil, fmt.Errorf("failed to create extractor for page %d: %w", i, err)
		}

		text, err := ex.ExtractText()
		if err != nil {
			return nil, fmt.Errorf("failed to extract text from page %d: %w", i, err)
		}

		fullText.WriteString(text)
		fullText.WriteString("\n")
	}

	rawText := fullText.String()
	if p.debug {
		fmt.Println("Extracted Text:")
		fmt.Println(rawText)
	}

	// Parse the extracted text
	vergiLevhasi := &VergiLevhasi{
		RawText: rawText,
	}

	p.parseContent(vergiLevhasi, rawText)

	return vergiLevhasi, nil
}

// parseContent extracts structured data from the raw text
func (p *Parser) parseContent(vl *VergiLevhasi, text string) {
	// Extract Adı Soyadı (Full Name)
	vl.AdiSoyadi = p.extractField(text, []string{
		`(?i)adı\s*soyadı\s*[:：]\s*(.+?)(?:\n|$)`,
		`(?i)adı\s+soyadı\s*[:：]\s*(.+?)(?:\n|$)`,
		`(?i)ad[ıi]\s*soyad[ıi]\s*[:：]?\s*(.+?)(?:\n|$)`,
	})

	// Extract Ticaret Ünvanı (Trade Name)
	vl.TicaretUnvani = p.extractField(text, []string{
		`(?i)ticaret\s*ünvanı\s*[:：]\s*(.+?)(?:\n|$)`,
		`(?i)ticaret\s+ünvan[ıi]\s*[:：]\s*(.+?)(?:\n|$)`,
		`(?i)ünvan[ıi]?\s*[:：]\s*(.+?)(?:\n|$)`,
	})

	// Extract İş Yeri Adresi (Business Address)
	vl.IsYeriAdresi = p.extractField(text, []string{
		`(?i)iş\s*yeri\s*adresi\s*[:：]\s*(.+?)(?:\n|$)`,
		`(?i)[iİ]ş\s*[yY]eri\s*[aA]dresi\s*[:：]\s*(.+?)(?:\n|$)`,
		`(?i)adres[ıi]?\s*[:：]\s*(.+?)(?:\n|$)`,
	})

	// Extract Vergi Dairesi (Tax Office)
	vl.VergiDairesi = p.extractField(text, []string{
		`(?i)vergi\s*dairesi\s*[:：]\s*(.+?)(?:\n|$)`,
		`(?i)vergi\s+dairesi\s*[:：]\s*(.+?)(?:\n|$)`,
	})

	// Extract Vergi Kimlik No (Tax ID Number)
	vl.VergiKimlikNo = p.extractField(text, []string{
		`(?i)vergi\s*kimlik\s*no\s*[:：]\s*(\d+)`,
		`(?i)vergi\s*kimlik\s*numaras[ıi]\s*[:：]\s*(\d+)`,
		`(?i)v\.k\.n\.?\s*[:：]\s*(\d+)`,
		`(?i)vkn\s*[:：]\s*(\d+)`,
	})

	// Extract TC Kimlik No (Turkish ID Number)
	vl.TCKimlikNo = p.extractField(text, []string{
		`(?i)t\.?c\.?\s*kimlik\s*no\s*[:：]\s*(\d+)`,
		`(?i)tc\s*kimlik\s*numaras[ıi]\s*[:：]\s*(\d+)`,
		`(?i)tckn\s*[:：]\s*(\d+)`,
	})

	// Extract İşe Başlama Tarihi (Business Start Date)
	dateStr := p.extractField(text, []string{
		`(?i)işe\s*başlama\s*tarihi\s*[:：]\s*(\d{2}[./-]\d{2}[./-]\d{4})`,
		`(?i)[iİ]şe\s*[bB]aşlama\s*[tT]arihi\s*[:：]\s*(\d{2}[./-]\d{2}[./-]\d{4})`,
		`(?i)başlama\s*tarihi\s*[:：]\s*(\d{2}[./-]\d{2}[./-]\d{4})`,
	})
	if dateStr != "" {
		if date, err := p.parseDate(dateStr); err == nil {
			vl.IseBaslamaTarihi = &date
		}
	}

	// Extract Vergi Türü (Tax Types)
	vl.VergiTuru = p.extractTaxTypes(text)

	// Extract Faaliyet Kodları (Activity Codes)
	vl.FaaliyetKodlari = p.extractActivities(text)

	// Extract Geçmiş Matrahlar (Historical Tax Bases)
	vl.GecmisMatra = p.extractTaxBases(text)
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

	// Pattern for activity codes (usually 4-6 digits followed by description)
	re := regexp.MustCompile(`(?m)(\d{4,6})\s*[-–]\s*(.{10,100})`)
	matches := re.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) > 2 {
			activities = append(activities, Faaliyet{
				Kod: strings.TrimSpace(match[1]),
				Ad:  strings.TrimSpace(match[2]),
			})
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
