package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	vergilevhasi "github.com/alparslanahmed/vergilevhasi-parser-go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Vergi Levhası Parser - Simple Example")
		fmt.Println("")
		fmt.Println("Usage: go run example/simple/main.go <path-to-pdf>")
		fmt.Println("")
		fmt.Println("Example:")
		fmt.Println("  go run example/simple/main.go vergi-levhasi.pdf")
		os.Exit(1)
	}

	pdfPath := os.Args[1]

	// Verify file exists
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		log.Fatalf("File not found: %s", pdfPath)
	}

	// Create a new parser
	parser := vergilevhasi.NewParser()

	// Parse the PDF
	fmt.Printf("Parsing: %s\n\n", pdfPath)
	result, err := parser.ParseFile(pdfPath)
	if err != nil {
		log.Fatalf("Failed to parse PDF: %v", err)
	}

	// Print the results as JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	fmt.Println(string(jsonData))
}
