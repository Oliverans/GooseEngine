package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"chess-engine/tuner" // Replace with your actual module path
)

func main() {
	input := flag.String("in", "", "Input TSV/CSV file")
	output := flag.String("out", "", "Output binary file")
	isCSV := flag.Bool("csv", false, "Input is CSV (default: TSV)")
	maxRows := flag.Int("max", 0, "Maximum rows to convert (0 = all)")

	flag.Parse()

	if *input == "" || *output == "" {
		fmt.Println("Usage: convert -in <input.book> -out <output.bin>")
		fmt.Println("Options:")
		fmt.Println("  -csv       Input is CSV format (default: TSV)")
		fmt.Println("  -max N     Convert only first N rows (default: all)")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Ensure output directory exists
	outDir := filepath.Dir(*output)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Convert
	if err := tuner.ConvertToBinary(*input, *output, *isCSV, *maxRows); err != nil {
		fmt.Fprintf(os.Stderr, "Conversion failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nConversion complete!")
}
