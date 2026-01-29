/*
Package vergilevhasi provides tools for parsing Turkish tax plate (Vergi Levhası) PDF documents.

This library extracts structured data from tax plate PDFs issued by the Turkish Revenue
Administration (Gelir İdaresi Başkanlığı - GİB).

# Basic Usage

	parser := vergilevhasi.NewParser()
	result, err := parser.ParseFile("vergi-levhasi.pdf")
	if err != nil {
	    log.Fatal(err)
	}
	fmt.Printf("VKN: %s\n", result.VergiKimlikNo)

# Extracted Fields

The parser extracts the following information:
  - Adı Soyadı (Full Name) - for individuals
  - Ticaret Ünvanı (Trade Name) - for companies
  - İş Yeri Adresi (Business Address)
  - Vergi Türü (Tax Types)
  - Faaliyet Kodları (Activity Codes - NACE codes)
  - Vergi Dairesi (Tax Office)
  - Vergi Kimlik No (Tax ID Number - VKN)
  - TC Kimlik No (Turkish ID Number - TCKN) - for individuals
  - İşe Başlama Tarihi (Business Start Date)
  - Geçmiş Matrahlar (Historical Tax Bases)

# OCR Support

For PDFs where the VKN is embedded as a barcode image rather than text,
use the OCR parser:

	parser, _ := vergilevhasi.NewOCRParser()
	defer parser.Close()
	vkn, err := parser.ExtractVKNFromImage("vergi-levhasi.png")
	// or from PDF bytes:
	// vkn, err := parser.ExtractVKNFromPDFBytes(pdfData)
*/
package vergilevhasi

import (
	"time"
)

// VergiLevhasi represents the structured data extracted from a Turkish tax plate PDF
type VergiLevhasi struct {
	// Adı Soyadı (Full Name) - for individuals, can be empty
	AdiSoyadi string `json:"adi_soyadi"`

	// Ticaret Ünvanı (Trade Name) - for companies, can be empty
	TicaretUnvani string `json:"ticaret_unvani"`

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
	Yil   int     `json:"yil"`
	Donem string  `json:"donem,omitempty"`
	Tutar float64 `json:"tutar,omitempty"`
	Tur   string  `json:"tur,omitempty"`
}
