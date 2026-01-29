package vergilevhasi

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"golang.org/x/text/encoding/charmap"
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
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Warning: Could not close file: %v", err)
		}
	}(file)

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

	// Create pdfcpu configuration
	conf := model.NewDefaultConfiguration()

	// Read, validate and optimize the PDF safely using pdfcpu
	ctx, err := api.ReadValidateAndOptimize(rs, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to read and validate PDF: %w", err)
	}

	// Extract text from all pages using pdfcpu's ExtractPageContent
	var rawText strings.Builder
	for pageNr := 1; pageNr <= ctx.PageCount; pageNr++ {
		contentReader, err := pdfcpu.ExtractPageContent(ctx, pageNr)
		if err != nil {
			continue
		}
		if contentReader == nil {
			continue
		}

		contentBytes, err := io.ReadAll(contentReader)
		if err != nil {
			continue
		}

		// Parse the PDF content stream to extract text
		pageText := extractTextFromPDFContent(string(contentBytes))
		rawText.WriteString(pageText)
		rawText.WriteString("\n")
	}

	// Combine extraction methods
	combinedText := rawText.String()
	ocrParser, err := NewOCRParser()
	if err != nil {
		log.Printf("Warning: Could not create OCR parser: %v", err)
	} else {
		defer func(ocrParser *OCRParser) {
			err := ocrParser.Close()
			if err != nil {
				log.Printf("Warning: Could not close OCR parser: %v", err)
			}
		}(ocrParser)
		ocrParser.SetOCRDebug(p.debug)
		vkn, err := ocrParser.ExtractVKNFromPDFWithImage(data)
		if err == nil && vkn != "" {
			combinedText += "\nVKN: " + vkn + "\n"
			fmt.Printf("VKN extracted via OCR: %s\n\n", vkn)
		} else if err != nil {
			log.Printf("OCR extraction failed: %v", err)
		}
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

// extractTextFromPDFContent parses PDF content stream operators to extract text
func extractTextFromPDFContent(content string) string {
	var result strings.Builder

	// PDF text is encoded between BT (begin text) and ET (end text)
	// Text showing operators include: Tj, TJ, ', "
	// We look for text in parentheses (literal strings) or angle brackets (hex strings)

	// Extract text from parenthesized strings using a parser that handles escapes
	extractedStrings := extractPDFStrings(content)
	for _, s := range extractedStrings {
		text := decodePDFString(s)
		result.WriteString(text)
		result.WriteString("\n")
	}

	// Pattern for hex strings
	hexRe := regexp.MustCompile(`<([0-9A-Fa-f]+)>`)
	hexMatches := hexRe.FindAllStringSubmatch(content, -1)
	for _, match := range hexMatches {
		if len(match) > 1 {
			text := decodeHexString(match[1])
			if text != "" {
				result.WriteString(text)
				result.WriteString("\nYILLIK GELİR VERGİSİ")
			}
		}
	}

	return result.String()
}

// extractPDFStrings extracts strings enclosed in parentheses, handling escaped parens
func extractPDFStrings(content string) []string {
	var results []string
	i := 0
	for i < len(content) {
		if content[i] == '(' {
			// Find matching closing parenthesis, handling escapes and nested parens
			str, endIdx := extractPDFString(content, i)
			if endIdx > i {
				results = append(results, str)
				i = endIdx
			} else {
				i++
			}
		} else {
			i++
		}
	}
	return results
}

// extractPDFString extracts a single parenthesized string starting at position start
// Returns the string content (without outer parens) and the index after the closing paren
func extractPDFString(content string, start int) (string, int) {
	if start >= len(content) || content[start] != '(' {
		return "", start
	}

	var result strings.Builder
	depth := 0
	i := start

	for i < len(content) {
		ch := content[i]
		if ch == '\\' && i+1 < len(content) {
			// Escaped character - include both backslash and next char in result
			result.WriteByte(ch)
			result.WriteByte(content[i+1])
			i += 2
			continue
		}
		if ch == '(' {
			depth++
			if depth > 1 {
				result.WriteByte(ch)
			}
		} else if ch == ')' {
			depth--
			if depth == 0 {
				return result.String(), i + 1
			}
			result.WriteByte(ch)
		} else if depth > 0 {
			result.WriteByte(ch)
		}
		i++
	}
	return result.String(), i
}

// decodePDFString decodes escape sequences in PDF literal strings
func decodePDFString(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				result.WriteRune('\n')
			case 'r':
				result.WriteRune('\r')
			case 't':
				result.WriteRune('\t')
			case 'b':
				result.WriteRune('\b')
			case 'f':
				result.WriteRune('\f')
			case '(':
				result.WriteRune('(')
			case ')':
				result.WriteRune(')')
			case '\\':
				result.WriteRune('\\')
			default:
				// Octal escape sequence
				if s[i+1] >= '0' && s[i+1] <= '7' {
					octal := string(s[i+1])
					j := i + 2
					for k := 0; k < 2 && j < len(s) && s[j] >= '0' && s[j] <= '7'; k++ {
						octal += string(s[j])
						j++
					}
					if val, err := strconv.ParseInt(octal, 8, 32); err == nil {
						result.WriteRune(rune(val))
					}
					i = j - 1
				} else {
					result.WriteByte(s[i+1])
				}
			}
			i += 2
		} else {
			result.WriteByte(s[i])
			i++
		}
	}

	// Try to convert from Windows-1254 (Turkish) to UTF-8 if needed
	decoded := result.String()
	if containsReplacementChars(decoded) || containsHighBytes(decoded) {
		if converted, err := convertWindows1254ToUTF8(decoded); err == nil {
			return converted
		}
	}
	return decoded
}

// containsReplacementChars checks if string contains Unicode replacement characters
func containsReplacementChars(s string) bool {
	return strings.ContainsRune(s, '\ufffd')
}

// containsHighBytes checks if string contains bytes > 127 (potential non-UTF8)
func containsHighBytes(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return true
		}
	}
	return false
}

// convertWindows1254ToUTF8 converts Windows-1254 encoded string to UTF-8
func convertWindows1254ToUTF8(s string) (string, error) {
	decoder := charmap.Windows1254.NewDecoder()
	result, err := decoder.String(s)
	if err != nil {
		return s, err
	}
	return result, nil
}

// decodeHexString decodes hex-encoded strings, including Turkish and other Unicode characters
func decodeHexString(hex string) string {
	// Pad with 0 if odd length
	if len(hex)%2 != 0 {
		hex += "0"
	}

	// Convert hex to bytes first
	byteData := make([]byte, len(hex)/2)
	for i := 0; i+1 < len(hex); i += 2 {
		val, err := strconv.ParseInt(hex[i:i+2], 16, 32)
		if err != nil {
			continue
		}
		byteData[i/2] = byte(val)
	}

	// Check for UTF-16BE BOM (FEFF) or detect UTF-16BE encoding
	if len(byteData) >= 2 && byteData[0] == 0xFE && byteData[1] == 0xFF {
		// UTF-16BE with BOM
		return decodeUTF16BE(byteData[2:])
	}

	// Check if this looks like UTF-16BE (alternating null bytes for ASCII range)
	if len(byteData) >= 4 && isLikelyUTF16BE(byteData) {
		return decodeUTF16BE(byteData)
	}

	// Try as single-byte encoding (Windows-1254 for Turkish)
	var result strings.Builder
	for _, b := range byteData {
		// Include printable ASCII and extended Latin characters
		if b >= 32 {
			result.WriteByte(b)
		}
	}

	decoded := result.String()
	// Convert from Windows-1254 to UTF-8 if it contains high bytes
	if containsHighBytes(decoded) {
		if converted, err := convertWindows1254ToUTF8(decoded); err == nil {
			return converted
		}
	}
	return decoded
}

// isLikelyUTF16BE checks if bytes look like UTF-16BE encoded text
func isLikelyUTF16BE(data []byte) bool {
	if len(data) < 4 || len(data)%2 != 0 {
		return false
	}
	// Check if odd positions (high bytes) are mostly zero for ASCII-like text
	zeroCount := 0
	for i := 0; i < len(data); i += 2 {
		if data[i] == 0 {
			zeroCount++
		}
	}
	// If more than 70% of high bytes are zero, it's likely UTF-16BE
	return zeroCount > len(data)/4
}

// decodeUTF16BE decodes UTF-16BE encoded bytes to UTF-8 string
func decodeUTF16BE(data []byte) string {
	if len(data)%2 != 0 {
		data = append(data, 0)
	}

	// Convert bytes to uint16 slice (big-endian)
	u16 := make([]uint16, len(data)/2)
	for i := 0; i < len(data); i += 2 {
		u16[i/2] = uint16(data[i])<<8 | uint16(data[i+1])
	}

	// Decode UTF-16 to runes
	runes := utf16.Decode(u16)

	// Build result string
	var result strings.Builder
	for _, r := range runes {
		if r >= 32 || r == '\n' || r == '\r' || r == '\t' {
			result.WriteRune(r)
		}
	}
	return result.String()
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

	// Try line-based parsing first for GIB PDF format
	p.parseLineBasedFormat(vl, lines, containsAny)

	// Try GIB single-line format parsing first
	// GIB PDFs often have all data in a single line with labels mixed with values
	p.parseGIBFormat(vl, text, containsAny)

	// Try traditional format only if GIB format didn't find the values (with colons)
	// Extract Adı Soyadı (Full Name) - traditional format with colon
	if vl.AdiSoyadi == "" {
		vl.AdiSoyadi = p.extractField(text, []string{
			`(?i)adı\s*soyadı\s*[:：]\s*(.+?)(?:\n|$)`,
			`(?i)ad[ıi]\s*soyad[ıi]\s*[:：]\s*(.+?)(?:\n|$)`,
		})
	}

	// Extract Ticaret Ünvanı - traditional format
	if vl.TicaretUnvani == "" {
		vl.TicaretUnvani = p.extractField(text, []string{
			`(?i)ticaret\s*ünvanı\s*[:：]\s*(.+?)(?:\n|$)`,
			`(?i)ticaret\s+ünvan[ıi]\s*[:：]\s*(.+?)(?:\n|$)`,
		})
	}

	// Extract İş Yeri Adresi - traditional format
	if vl.IsYeriAdresi == "" {
		vl.IsYeriAdresi = p.extractField(text, []string{
			`(?i)iş\s*yeri\s*adresi\s*[:：]\s*(.+?)(?:\n|$)`,
			`(?i)[iİ]ş\s*[yY]eri\s*[aA]dresi\s*[:：]\s*(.+?)(?:\n|$)`,
		})
	}

	// Extract Vergi Dairesi - traditional format
	if vl.VergiDairesi == "" {
		vl.VergiDairesi = p.extractField(text, []string{
			`(?i)vergi\s*dairesi\s*[:：]\s*(.+?)(?:\n|$)`,
		})
	}

	// Extract Vergi Kimlik No - traditional format
	if vl.VergiKimlikNo == "" {
		vl.VergiKimlikNo = p.extractField(text, []string{
			`(?i)vergi\s*kimlik\s*no\s*[:：]\s*(\d{10})`,
			`(?i)v\.?k\.?n\.?\s*[:：]\s*(\d{10})`,
		})
	}

	// Extract TC Kimlik No - traditional format
	if vl.TCKimlikNo == "" {
		vl.TCKimlikNo = p.extractField(text, []string{
			`(?i)t\.?c\.?\s*kimlik\s*no\s*[:：]\s*(\d{11})`,
			`(?i)tckn\s*[:：]\s*(\d{11})`,
			`(?i)tc\s*k[iİ]ml[iİ]k\s*no\s*[:：]?\s*(\d{11})`,
			`(?i)t\.c\.\s*k[iİ]ml[iİ]k\s*no\s*[:：]?\s*(\d{11})`,
		})
	}

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
			hasAddressMarker := containsAny(line, "MAH.", "MAH ", "CAD.", "CAD ", "SOK.", "SOK ", "KAPI NO", "İÇ KAPI", "IC KAPI", "YOLU CAD")
			// Also check for "NO:" but only if preceded by address-related words
			if !hasAddressMarker && strings.Contains(strings.ToUpper(line), "NO:") {
				// Check if this looks like an address (has building/apartment indicators)
				if containsAny(line, "KAPI", "DAİRE", "DAIRE", "KAT", "BLOK", "TOWER", "PLAZA", "RESIDENCE", "KONUT") {
					hasAddressMarker = true
				}
			}
			// Check for famous building names that indicate addresses
			if !hasAddressMarker {
				if containsAny(line, "TRUMP TOWER", "TOWER", "PLAZA", "CENTER", "CENTRE", "İŞ MERKEZİ", "IS MERKEZI") {
					hasAddressMarker = true
				}
			}

			if hasAddressMarker {
				// Exclude activity code lines and ID number lines
				if !containsAny(line, "FAALİYET", "FAALIYET", "KİMLİK", "KIMLIK", "TC KİMLİK", "TC KIMLIK") {
					if vl.IsYeriAdresi == "" {
						vl.IsYeriAdresi = trimmedLine
						// Only take the first address line to avoid duplicates
						break
					}
				}
			}
		}
	}

	// Vergi Dairesi is extracted by parseLineBasedFormat using position-based logic
	// (between tax type line and date/TCKN line)

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

	isKurumsal := false
	for _, vt := range vl.VergiTuru {
		if strings.Contains(strings.ToLower(vt), "kurumlar") {
			isKurumsal = true
			break
		}
	}

	if isKurumsal {
		// Kurumsal: AdiSoyadi'ndaki değer aslında TicaretUnvani olmalı
		if vl.AdiSoyadi != "" && vl.TicaretUnvani == "" {
			vl.TicaretUnvani = vl.AdiSoyadi
			vl.AdiSoyadi = ""
		}
		// For corporate taxpayers, clear AdiSoyadi if TicaretUnvani is set
		// (corporations don't have personal names, only trade names)
		if vl.TicaretUnvani != "" {
			vl.AdiSoyadi = ""
		}
	} else {
		// Bireysel: TicaretUnvani boş olmalı (bireysel mükellefin ticaret unvanı yok)
		// AdiSoyadi zaten doğru yerde
		if vl.TicaretUnvani != "" && vl.AdiSoyadi == "" {
			// Eğer sadece TicaretUnvani varsa ve bireysel ise, bu aslında isim
			vl.AdiSoyadi = vl.TicaretUnvani
			vl.TicaretUnvani = ""
		}
	}
}

// parseLineBasedFormat parses the GIB PDF using line-based logic
// This handles the specific structure where:
// - Lines 13-14 contain "FAALİYET KOD VE ADLARI" or "ANA FAALİYET KODU VE ADI"
// - Line 15 contains "MÜKELLEFİN"
// - Lines 16-17 contain company/person name (may be 1 or 2 lines)
// - Next lines contain address (may be 1 or 2 lines)
// - Then comes tax type (e.g., "KURUMLAR VERGİSİ" or "YILLIK GELİR VERGİSİ")
// - Then comes tax office (Vergi Dairesi)
func (p *Parser) parseLineBasedFormat(vl *VergiLevhasi, lines []string, containsAny func(string, ...string) bool) {
	// Turkish city names for detecting second address line
	turkishCities := []string{
		"ADANA", "ADIYAMAN", "AFYONKARAHİSAR", "AĞRI", "AMASYA", "ANKARA", "ANTALYA", "ARTVİN",
		"AYDIN", "BALIKESİR", "BİLECİK", "BİNGÖL", "BİTLİS", "BOLU", "BURDUR", "BURSA",
		"ÇANAKKALE", "ÇANKIRI", "ÇORUM", "DENİZLİ", "DİYARBAKIR", "EDİRNE", "ELAZIĞ", "ERZİNCAN",
		"ERZURUM", "ESKİŞEHİR", "GAZİANTEP", "GİRESUN", "GÜMÜŞHANE", "HAKKARİ", "HATAY", "ISPARTA",
		"MERSİN", "İSTANBUL", "ISTANBUL", "İZMİR", "IZMIR", "KARS", "KASTAMONU", "KAYSERİ", "KIRKLARELİ",
		"KIRŞEHİR", "KOCAELİ", "KONYA", "KÜTAHYA", "MALATYA", "MANİSA", "KAHRAMANMARAŞ", "MARDİN",
		"MUĞLA", "MUŞ", "NEVŞEHİR", "NİĞDE", "ORDU", "RİZE", "SAKARYA", "SAMSUN", "SİİRT", "SİNOP",
		"SİVAS", "TEKİRDAĞ", "TOKAT", "TRABZON", "TUNCELİ", "ŞANLIURFA", "UŞAK", "VAN", "YOZGAT",
		"ZONGULDAK", "AKSARAY", "BAYBURT", "KARAMAN", "KIRIKKALE", "BATMAN", "ŞIRNAK", "BARTIN",
		"ARDAHAN", "IĞDIR", "YALOVA", "KARABÜK", "KİLİS", "OSMANİYE", "DÜZCE",
	}

	// Find "MÜKELLEFİN" line index
	mukellefinIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check for MÜKELLEFİN or MKELLEFIN (without Ü due to encoding issues)
		if strings.Contains(strings.ToUpper(trimmed), "MKELLEF") || strings.Contains(strings.ToUpper(trimmed), "MÜKELLEFİN") {
			mukellefinIdx = i
			break
		}
	}

	if mukellefinIdx == -1 || mukellefinIdx+1 >= len(lines) {
		return
	}

	// Address markers to check if a line is an address line
	addressMarkers := []string{"MAH.", "MAH ", "CAD.", "CAD ", "SOK.", "SOK ", "SK.", "SK ", "NO:", "KAPI", "BULVARI", "BULV."}
	isAddressLine := func(line string) bool {
		upperLine := strings.ToUpper(line)
		for _, marker := range addressMarkers {
			if strings.Contains(upperLine, marker) {
				return true
			}
		}
		return false
	}

	// Check if line contains a Turkish city name (for second address line detection)
	containsCityName := func(line string) bool {
		upperLine := strings.ToUpper(line)
		for _, city := range turkishCities {
			// Check for city name at end of line or followed by common patterns
			if strings.Contains(upperLine, "/ "+city) || strings.Contains(upperLine, "/"+city) ||
				strings.HasSuffix(upperLine, city) || strings.Contains(upperLine, city+"/") {
				return true
			}
		}
		return false
	}

	// Extract company/person name starting from line after MÜKELLEFİN
	nameStartIdx := mukellefinIdx + 1
	var nameLines []string
	var addressStartIdx int

	for i := nameStartIdx; i < len(lines) && i < nameStartIdx+3; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}

		// If this line looks like an address, stop collecting name lines
		if isAddressLine(trimmed) {
			addressStartIdx = i
			break
		}

		// This line is part of the name
		nameLines = append(nameLines, trimmed)
		addressStartIdx = i + 1
	}

	// Join name lines to form full company/person name
	if len(nameLines) > 0 {
		fullName := strings.Join(nameLines, " ")

		// Determine if this is a company or individual
		isCompany := containsAny(fullName, "ŞİRKET", "SIRKET", "LİMİTED", "LIMITED", "A.Ş", "A.S.",
			"DERNEĞİ", "DERNEGI", "İKTİSADİ", "IKTISADI", "SANAYİ", "SANAYI", "TİCARET", "TICARET")

		if isCompany {
			vl.TicaretUnvani = fullName
		} else {
			vl.AdiSoyadi = fullName
		}
	}

	// Extract address starting from addressStartIdx
	var addressLines []string
	var vergiTuruIdx int

	for i := addressStartIdx; i < len(lines) && i < addressStartIdx+3; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}

		// Check if this line is a tax type line (end of address section)
		if containsAny(trimmed, "KURUMLAR VERGİSİ", "YILLIK GELİR VERGİSİ", "GELİR VERGİSİ", "KATMA DEĞER VERGİSİ") {
			vergiTuruIdx = i
			break
		}

		// First address line or second line with city name
		if len(addressLines) == 0 && isAddressLine(trimmed) {
			addressLines = append(addressLines, trimmed)
		} else if len(addressLines) == 1 && containsCityName(trimmed) {
			// This is the second line of address containing city
			addressLines = append(addressLines, trimmed)
		} else if len(addressLines) >= 1 {
			// Not a city line, this might be vergi türü or something else
			vergiTuruIdx = i
			break
		}
	}

	// Join address lines
	if len(addressLines) > 0 {
		vl.IsYeriAdresi = strings.Join(addressLines, " ")
	}

	// Find Vergi Türü line index if not already found
	if vergiTuruIdx == 0 {
		for i := addressStartIdx; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if containsAny(trimmed, "KURUMLAR VERGİSİ", "YILLIK GELİR VERGİSİ", "GELİR VERGİSİ") {
				vergiTuruIdx = i
				break
			}
		}
	}

	// Extract Vergi Dairesi - it comes after tax type
	// For corporations: between "X VERGİSİ" and date (DD.MM.YYYY)
	// For individuals: between "X VERGİSİ" and 11-digit TCKN
	if vergiTuruIdx > 0 && vergiTuruIdx+1 < len(lines) {
		dateRe := regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}$`)
		tcknRe := regexp.MustCompile(`^\d{11}$`)
		digitOnlyRe := regexp.MustCompile(`^\d+$`)

		for i := vergiTuruIdx + 1; i < len(lines) && i < vergiTuruIdx+5; i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" {
				continue
			}

			// If we hit a date or TCKN, we've passed the vergi dairesi
			if dateRe.MatchString(trimmed) || tcknRe.MatchString(trimmed) {
				break
			}

			// Skip if this is a number pattern (could be VKN or other ID)
			if digitOnlyRe.MatchString(trimmed) {
				continue
			}

			// Skip if this is a tax type line
			if containsAny(trimmed, "VERGİSİ", "VERGISI") {
				continue
			}

			// This should be the vergi dairesi
			if len(trimmed) > 2 && !containsAny(trimmed, "http", "www", "gib.gov") {
				vl.VergiDairesi = trimmed
				break
			}
		}
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

// parseGIBFormat parses the GIB (Revenue Administration) PDF format
// This format often has all content in a single line with labels and values mixed
func (p *Parser) parseGIBFormat(vl *VergiLevhasi, text string, containsAny func(string, ...string) bool) {
	// GIB format has a specific structure:
	// Labels come first, then values in the same order
	// Example: "VERGİ LEVHASI ADI SOYADI TİCARET ÜNVANI İŞ YERİ ADRESİ ... MÜKELLEFİN [NAME] [ADDRESS] MAH..."

	// Check if this is GIB format (contains "VERGİ LEVHASI" or similar markers)
	if !containsAny(text, "VERGİ LEVHASI", "VERGI LEVHASI", "GİB", "GIB") {
		return
	}

	// Extract name - look for "MÜKELLEFİN" followed by name until "MAH" (mahalle)
	// Pattern: MÜKELLEFİN [NAME] [DISTRICT] MAH...
	// Only set if not already set by line-based parsing
	if vl.AdiSoyadi == "" && vl.TicaretUnvani == "" {
		nameRe := regexp.MustCompile(`MÜKELLEFİN\s+(.+?)\s+[A-ZÇĞİÖŞÜ]+\s+MAH`)
		if matches := nameRe.FindStringSubmatch(text); len(matches) > 1 {
			name := strings.TrimSpace(matches[1])
			if len(name) > 3 {
				vl.AdiSoyadi = name
			}
		}
	}

	addrRe := regexp.MustCompile(`([A-ZÇĞİÖŞÜ]+\s+MAH\.?\s+.+?(?:İSTANBUL|ISTANBUL|ANKARA|İZMİR|IZMIR|BURSA|ANTALYA|KONYA))`)
	if matches := addrRe.FindStringSubmatch(text); len(matches) > 1 {
		addr := strings.TrimSpace(matches[1])
		// Remove trailing tax type if captured
		if idx := strings.Index(addr, " YILLIK"); idx > 0 {
			addr = strings.TrimSpace(addr[:idx])
		}
		if idx := strings.Index(addr, " KURUMLAR"); idx > 0 {
			addr = strings.TrimSpace(addr[:idx])
		}
		if len(addr) > 20 {
			vl.IsYeriAdresi = addr
		}
	}

	// Extract Vergi Dairesi - between tax type and 11-digit TCKN
	// Pattern: YILLIK GELİR VERGİSİ [TAX_OFFICE] [11_DIGIT_TCKN]
	// Only set if not already set by line-based parsing
	if vl.VergiDairesi == "" {
		taxOfficeRe := regexp.MustCompile(`(?:YILLIK\s+GELİR\s+VERGİSİ|GELİR\s+VERGİSİ|KURUMLAR\s+VERGİSİ)\s+([A-ZÇĞİÖŞÜ]+)\s+\d{11}`)
		if matches := taxOfficeRe.FindStringSubmatch(text); len(matches) > 1 {
			vl.VergiDairesi = strings.TrimSpace(matches[1])
		}
	}

	// Extract VKN (10-digit) - not applicable for bireysel, they have 11-digit TCKN
	// VKN is for kurumsal only

	// Extract date - look for DD.MM.YYYY pattern
	dateRe := regexp.MustCompile(`(\d{2}\.\d{2}\.\d{4})`)
	if matches := dateRe.FindStringSubmatch(text); len(matches) > 1 {
		if date, err := p.parseDate(matches[1]); err == nil {
			vl.IseBaslamaTarihi = &date
		}
	}

	// Extract activity code and name - look for 6-digit code followed by dash and description
	activityRe := regexp.MustCompile(`(\d{6})\s*[-–]\s*([A-ZÇĞİÖŞÜa-zçğıöşü\s]+?)(?:\s+TAKVİM|\s+TAKVIM|\s+BEYAN|\s+\d{4})`)
	if matches := activityRe.FindStringSubmatch(text); len(matches) > 2 {
		vl.FaaliyetKodlari = []Faaliyet{{
			Kod: strings.TrimSpace(matches[1]),
			Ad:  strings.TrimSpace(matches[2]),
		}}
	}
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

	textLower := strings.ToLower(text)
	seen := make(map[string]bool)

	// Check for specific tax types in order (more specific first)
	// This ensures "Yıllık Gelir Vergisi" is checked before "Gelir Vergisi"
	taxTypeChecks := []struct {
		pattern     string
		displayName string
	}{
		{"yıllık gelir vergisi", "Yıllık Gelir Vergisi"},
		{"yillik gelir vergisi", "Yıllık Gelir Vergisi"},
		{"kurumlar vergisi", "Kurumlar Vergisi"},
		{"katma değer vergisi", "Katma Değer Vergisi"},
		{"katma deger vergisi", "Katma Değer Vergisi"},
		{"geçici vergi", "Geçici Vergi"},
		{"gecici vergi", "Geçici Vergi"},
		{"damga vergisi", "Damga Vergisi"},
		{"muhtasar", "Muhtasar"},
		{"stopaj", "Stopaj"},
		{"bağ-kur", "Bağ-Kur"},
		{"bag-kur", "Bağ-Kur"},
		{"sgk", "SGK"},
		{"kdv", "KDV"},
		// "Gelir Vergisi" checked last - only if Yıllık not found
		{"gelir vergisi", "Gelir Vergisi"},
	}

	for _, check := range taxTypeChecks {
		if strings.Contains(textLower, check.pattern) && !seen[check.displayName] {
			// For "Gelir Vergisi", skip if "Yıllık Gelir Vergisi" is already added
			if check.displayName == "Gelir Vergisi" && seen["Yıllık Gelir Vergisi"] {
				continue
			}
			seen[check.displayName] = true
			types = append(types, check.displayName)
		}
	}

	return types
}

// extractActivities extracts activity codes and names
func (p *Parser) extractActivities(text string) []Faaliyet {
	var activities []Faaliyet
	seen := make(map[string]bool)

	// Split by lines and process each line
	lines := strings.Split(text, "\n")

	// Pattern for activity codes (usually 4-6 digits followed by description)
	// We process line by line for better control
	lineRe := regexp.MustCompile(`(\d{4,6})\s*[-–]\s*(.+)`)

	for _, line := range lines {
		matches := lineRe.FindStringSubmatch(line)
		if len(matches) > 2 {
			kod := strings.TrimSpace(matches[1])
			ad := strings.TrimSpace(matches[2])

			// Clean up activity name - remove common suffixes
			cleanupPatterns := []string{"TAKVİM", "TAKVIM", "BEYAN", "ONAY", "MATRAH"}
			for _, pattern := range cleanupPatterns {
				if idx := strings.Index(strings.ToUpper(ad), pattern); idx > 0 {
					ad = strings.TrimSpace(ad[:idx])
				}
			}

			// Remove year patterns at the end (e.g., "2024 Ma")
			yearRe := regexp.MustCompile(`\s+\d{4}\s*[A-Za-z]*$`)
			ad = yearRe.ReplaceAllString(ad, "")
			ad = strings.TrimSpace(ad)

			if !seen[kod] && len(ad) > 3 {
				seen[kod] = true
				activities = append(activities, Faaliyet{
					Kod: kod,
					Ad:  ad,
				})
			}
		}
	}

	// Also try to find activities in single-line format (GIB PDFs)
	if len(activities) == 0 {
		re := regexp.MustCompile(`(\d{6})\s*[-–]\s*([A-ZÇĞİÖŞÜa-zçğıöşü\s]+?)(?:\s+TAKVİM|\s+TAKVIM|\s+BEYAN|\s+\d{4})`)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 2 {
				kod := strings.TrimSpace(match[1])
				ad := strings.TrimSpace(match[2])
				if !seen[kod] && len(ad) > 3 {
					seen[kod] = true
					activities = append(activities, Faaliyet{
						Kod: kod,
						Ad:  ad,
					})
				}
			}
		}
	}

	return activities
}

// extractTaxBases extracts historical tax base information
func (p *Parser) extractTaxBases(text string) []Matrah {
	var matrahlar []Matrah

	// Check if the document indicates "Matrahsız" (no tax base)
	// In this case, we should not extract any amounts
	textUpper := strings.ToUpper(text)
	if strings.Contains(textUpper, "MATRAHSIZ") {
		// Document indicates no tax base data, return empty
		return matrahlar
	}

	// Pattern for year and amount - must be a realistic tax amount (at least 4 digits)
	// This prevents matching activity codes (621000) or small numbers
	// Matches: "2020 100.000,00" or "2020 yılı 100.000,00 TL"
	re := regexp.MustCompile(`(?m)(\d{4})\s+(?:yılı\s+)?(\d{1,3}(?:[.,]\d{3})+(?:[.,]\d{2})?)\s*(?:TL|₺)?`)
	matches := re.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) > 2 {
			year, err := strconv.Atoi(match[1])
			if err != nil || year < 2000 || year > 2100 {
				continue
			}

			// Parse amount
			amountStr := strings.ReplaceAll(match[2], ".", "")
			amountStr = strings.ReplaceAll(amountStr, ",", ".")
			amount, err := strconv.ParseFloat(amountStr, 64)
			if err != nil {
				continue
			}

			// Skip unrealistically small amounts (likely parsing errors)
			// Real tax bases are typically at least 1000 TL
			if amount < 1000 {
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
