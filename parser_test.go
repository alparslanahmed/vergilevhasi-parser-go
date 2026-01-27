package vergilevhasi

import (
	"strings"
	"testing"
	"time"
)

func TestNewParser(t *testing.T) {
	parser := NewParser()
	if parser == nil {
		t.Fatal("NewParser returned nil")
	}
}

func TestSetDebug(t *testing.T) {
	parser := NewParser()
	parser.SetDebug(true)
	if !parser.debug {
		t.Error("SetDebug(true) failed to enable debug mode")
	}
	parser.SetDebug(false)
	if parser.debug {
		t.Error("SetDebug(false) failed to disable debug mode")
	}
}

func TestExtractField(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		text     string
		patterns []string
		want     string
	}{
		{
			name:     "Extract name",
			text:     "Adı Soyadı: Ali Örnek\n",
			patterns: []string{`(?i)adı\s*soyadı\s*[:：]\s*(.+?)(?:\n|$)`},
			want:     "Ali Örnek",
		},
		{
			name:     "Extract tax office",
			text:     "Vergi Dairesi: Örnek VD\n",
			patterns: []string{`(?i)vergi\s*dairesi\s*[:：]\s*(.+?)(?:\n|$)`},
			want:     "Örnek VD",
		},
		{
			name:     "No match",
			text:     "Some random text",
			patterns: []string{`(?i)adı\s*soyadı\s*[:：]\s*(.+?)(?:\n|$)`},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.extractField(tt.text, tt.patterns)
			if got != tt.want {
				t.Errorf("extractField() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name    string
		dateStr string
		wantErr bool
	}{
		{
			name:    "Valid date DD.MM.YYYY",
			dateStr: "15.06.2020",
			wantErr: false,
		},
		{
			name:    "Valid date DD/MM/YYYY",
			dateStr: "15/06/2020",
			wantErr: false,
		},
		{
			name:    "Valid date D.M.YYYY",
			dateStr: "5.6.2020",
			wantErr: false,
		},
		{
			name:    "Invalid date",
			dateStr: "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.parseDate(tt.dateStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtractTaxTypes(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "Single tax type",
			text: "Bu mükellefte Gelir Vergisi bulunmaktadır",
			want: []string{"Gelir Vergisi"},
		},
		{
			name: "Multiple tax types",
			text: "Gelir Vergisi ve KDV mükellefiyeti vardır",
			want: []string{"Gelir Vergisi", "KDV"},
		},
		{
			name: "No tax types",
			text: "Random text without tax types",
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.extractTaxTypes(tt.text)
			if len(got) != len(tt.want) {
				t.Errorf("extractTaxTypes() length = %v, want %v (got: %v)", len(got), len(tt.want), got)
				return
			}
			// Check that all expected types are present (order independent)
			for _, want := range tt.want {
				found := false
				for _, g := range got {
					if g == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("extractTaxTypes() missing %v, got %v", want, got)
				}
			}
		})
	}
}

func TestExtractActivities(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name string
		text string
		want int // number of activities expected
	}{
		{
			name: "Single activity",
			text: "4711 - Gıda, içecek ve tütün satışı",
			want: 1,
		},
		{
			name: "Multiple activities",
			text: "4711 - Gıda satışı\n5610 - Lokanta hizmetleri",
			want: 2,
		},
		{
			name: "No activities",
			text: "Random text without activity codes",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.extractActivities(tt.text)
			if len(got) != tt.want {
				t.Errorf("extractActivities() returned %d activities, want %d", len(got), tt.want)
			}
		})
	}
}

func TestExtractTaxBases(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name string
		text string
		want int // number of tax bases expected
	}{
		{
			name: "Single year",
			text: "2020 yılı 100.000,00 TL",
			want: 1,
		},
		{
			name: "Multiple years",
			text: "2019 150.000,00\n2020 200.000,00",
			want: 2,
		},
		{
			name: "No tax bases",
			text: "Random text",
			want: 0,
		},
		{
			name: "Matrahsız",
			text: "2020 Matrahsız 2021 Matrahsız",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.extractTaxBases(tt.text)
			if len(got) != tt.want {
				t.Errorf("extractTaxBases() returned %d tax bases, want %d", len(got), tt.want)
			}
		})
	}
}

func TestParseContent(t *testing.T) {
	parser := NewParser()

	// Using clearly fictional dummy data for testing
	text := `
	Adı Soyadı: Ali Örnek
	TC Kimlik No: 11111111110
	Vergi Kimlik No: 1234567890
	Vergi Dairesi: Örnek VD
	İş Yeri Adresi: Örnek Mah. Test Cad. No:1, Ankara
	İşe Başlama Tarihi: 01.01.2020
	Gelir Vergisi
	KDV
	4711 - Gıda, içecek ve tütün satışı
	2020 150.000,00 TL
	`

	vl := &VergiLevhasi{}
	parser.parseContent(vl, text)

	if vl.AdiSoyadi != "Ali Örnek" {
		t.Errorf("AdiSoyadi = %v, want 'Ali Örnek'", vl.AdiSoyadi)
	}

	if vl.TCKimlikNo != "11111111110" {
		t.Errorf("TCKimlikNo = %v, want '11111111110'", vl.TCKimlikNo)
	}

	if vl.VergiKimlikNo != "1234567890" {
		t.Errorf("VergiKimlikNo = %v, want '1234567890'", vl.VergiKimlikNo)
	}

	if vl.VergiDairesi != "Örnek VD" {
		t.Errorf("VergiDairesi = %v, want 'Örnek VD'", vl.VergiDairesi)
	}

	if !strings.Contains(vl.IsYeriAdresi, "Örnek") {
		t.Errorf("IsYeriAdresi = %v, want to contain 'Örnek'", vl.IsYeriAdresi)
	}

	if vl.IseBaslamaTarihi == nil {
		t.Error("IseBaslamaTarihi is nil")
	} else {
		expectedDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		if !vl.IseBaslamaTarihi.Equal(expectedDate) {
			t.Errorf("IseBaslamaTarihi = %v, want %v", vl.IseBaslamaTarihi, expectedDate)
		}
	}

	if len(vl.VergiTuru) < 2 {
		t.Errorf("VergiTuru length = %v, want at least 2", len(vl.VergiTuru))
	}

	if len(vl.FaaliyetKodlari) < 1 {
		t.Errorf("FaaliyetKodlari length = %v, want at least 1", len(vl.FaaliyetKodlari))
	}

	if len(vl.GecmisMatra) < 1 {
		t.Errorf("GecmisMatra length = %v, want at least 1", len(vl.GecmisMatra))
	}
}
