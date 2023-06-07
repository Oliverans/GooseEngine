package engine

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

func getOpeningMove() {
	p, _ := filepath.Abs("engine/opening_book.txt")
	_ = p
	file, err := os.Open(p)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	for {
		records, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		regexpToMatch := regexp.MustCompile(`([0-9]\.)`)
		result := regexpToMatch.ReplaceAllString(records[2], "")
		fmt.Println(result)
	}
}
