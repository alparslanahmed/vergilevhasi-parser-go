//go:build ignore

package main

import (
	"fmt"
	"os"

	vlp "github.com/alparslanahmed/vergilevhasi-parser-go"
)

func main() {
	if len(os.Args) < 2 {
		writeToFile("Usage: go run debug_vkn.go <pdf-file>\n")
		os.Exit(1)
	}

	pdfPath := os.Args[1]

	parser, err := vlp.NewOCRParser()
	if err != nil {
		writeToFile(fmt.Sprintf("Error creating parser: %v\n", err))
		os.Exit(1)
	}
	parser.SetOCRDebug(true)

	writeToFile("=== Testing ExtractVKNFromPDFWithImage ===\n")
	vkn, err := parser.ExtractVKNFromPDFWithImage(pdfPath)
	writeToFile(fmt.Sprintf("Result: VKN=%s, Error=%v\n", vkn, err))
}

func writeToFile(s string) {
	f, _ := os.OpenFile("/tmp/debug_vkn_output.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.WriteString(s)
	fmt.Print(s) // Also print to stdout
}
