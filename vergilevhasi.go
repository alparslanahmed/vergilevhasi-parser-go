package vergilevhasi

import (
	"time"
)

// VergiLevhasi represents the structured data extracted from a Turkish tax plate PDF
type VergiLevhasi struct {
	// Adı Soyadı (Full Name) - for individuals
	AdiSoyadi string `json:"adi_soyadi,omitempty"`

	// Ticaret Ünvanı (Trade Name) - for companies
	TicaretUnvani string `json:"ticaret_unvani,omitempty"`

	// İş Yeri Adresi (Business Address)
	IsYeriAdresi string `json:"is_yeri_adresi,omitempty"`

	// Vergi Türü (Tax Type)
	VergiTuru []string `json:"vergi_turu,omitempty"`

	// Faaliyet Kodları ve Adları (Activity Codes and Names)
	FaaliyetKodlari []Faaliyet `json:"faaliyet_kodlari,omitempty"`

	// Vergi Dairesi (Tax Office)
	VergiDairesi string `json:"vergi_dairesi,omitempty"`

	// Vergi Kimlik No (Tax ID Number)
	VergiKimlikNo string `json:"vergi_kimlik_no,omitempty"`

	// TC Kimlik No (Turkish ID Number) - for individuals
	TCKimlikNo string `json:"tc_kimlik_no,omitempty"`

	// İşe Başlama Tarihi (Business Start Date)
	IseBaslamaTarihi *time.Time `json:"ise_baslama_tarihi,omitempty"`

	// Geçmiş Matrahlar (Historical Tax Bases)
	GecmisMatra []Matrah `json:"gecmis_matrahlar,omitempty"`

	// Raw text extracted from PDF
	RawText string `json:"-"`
}

// Faaliyet represents an activity code and name
type Faaliyet struct {
	Kod string `json:"kod"`
	Ad  string `json:"ad"`
}

// Matrah represents historical tax base information
type Matrah struct {
	Yil    int     `json:"yil"`
	Donem  string  `json:"donem,omitempty"`
	Tutar  float64 `json:"tutar,omitempty"`
	Tur    string  `json:"tur,omitempty"`
}
