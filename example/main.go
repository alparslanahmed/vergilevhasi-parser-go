package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/alparslanahmed/vergilevhasi-parser-go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Vergi Levhası Parser - Example Application")
		fmt.Println("")
		fmt.Println("Usage: go run -tags ocr example/main.go <path-to-pdf>")
		fmt.Println("")
		fmt.Println("Example:")
		fmt.Println("  go run -tags ocr example/main.go vergi-levhasi.pdf")
		os.Exit(1)
	}

	pdfPath := os.Args[1]

	// Verify file exists
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		log.Fatalf("File not found: %s", pdfPath)
	}

	// Create a new parser
	parser := vergilevhasi.NewParser()
	parser.SetDebug(false) // Set to true to see extracted text

	// Parse the PDF
	fmt.Printf("Parsing: %s\n\n", pdfPath)
	result, err := parser.ParseFile(pdfPath)
	if err != nil {
		log.Fatalf("Failed to parse PDF: %v", err)
	}

	fmt.Println(result.VergiKimlikNo)
	// If VKN was not found via text extraction, try barcode scanning with OCR
	if result.VergiKimlikNo == "" {
		fmt.Println("VKN not found in text, attempting OCR extraction...")
		ocrParser, err := vergilevhasi.NewOCRParser()
		if err != nil {
			log.Printf("Warning: Could not create OCR parser: %v", err)
		} else {
			defer ocrParser.Close()
			vkn, err := ocrParser.ExtractVKNFromPDFWithImage(pdfPath)
			if err == nil && vkn != "" {
				result.VergiKimlikNo = vkn
				fmt.Printf("VKN extracted via OCR: %s\n\n", vkn)
			} else if err != nil {
				log.Printf("OCR extraction failed: %v", err)
			}
		}
	}

	// Print the results as JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	fmt.Println("=== JSON Output ===")
	fmt.Println(string(jsonData))

	// Print specific fields in a readable format
	fmt.Println("\n=== Extracted Information ===")
	if result.AdiSoyadi != "" {
		fmt.Printf("Adı Soyadı:          %s\n", result.AdiSoyadi)
	}
	if result.TicaretUnvani != "" {
		fmt.Printf("Ticaret Ünvanı:      %s\n", result.TicaretUnvani)
	}
	if result.VergiKimlikNo != "" {
		fmt.Printf("Vergi Kimlik No:     %s\n", result.VergiKimlikNo)
	}
	if result.TCKimlikNo != "" {
		fmt.Printf("TC Kimlik No:        %s\n", result.TCKimlikNo)
	}
	if result.VergiDairesi != "" {
		fmt.Printf("Vergi Dairesi:       %s\n", result.VergiDairesi)
	}
	if result.IsYeriAdresi != "" {
		fmt.Printf("İş Yeri Adresi:      %s\n", result.IsYeriAdresi)
	}
	if result.IseBaslamaTarihi != nil {
		fmt.Printf("İşe Başlama Tarihi:  %s\n", result.IseBaslamaTarihi.Format("02.01.2006"))
	}

	if len(result.VergiTuru) > 0 {
		fmt.Println("\nVergi Türleri:")
		for _, vt := range result.VergiTuru {
			fmt.Printf("  • %s\n", vt)
		}
	}

	if len(result.FaaliyetKodlari) > 0 {
		fmt.Println("\nFaaliyet Kodları:")
		for _, fk := range result.FaaliyetKodlari {
			fmt.Printf("  • %s: %s\n", fk.Kod, fk.Ad)
		}
	}

	if len(result.GecmisMatra) > 0 {
		fmt.Println("\nGeçmiş Matrahlar:")
		for _, gm := range result.GecmisMatra {
			fmt.Printf("  • %d: %.2f TL\n", gm.Yil, gm.Tutar)
		}
	}
}
