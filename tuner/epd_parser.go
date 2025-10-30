package tuner

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func parseNextEPD(entries *[]TEntry) {
	//file, err := os.OpenFile("./tuner/tuning_positions.epd", os.O_RDONLY, 0600)
	file, err := os.OpenFile("./tuner/E12.41-1M-D12-Resolved.book", os.O_RDONLY, 0600)
	check(err)
	defer file.Close()

	sc := bufio.NewScanner(file)
	linesScanned := 0

	for sc.Scan() {
		line := sc.Text()
		parts := strings.Split(line, "[")
		if len(parts) != 2 {
			continue // skip malformed lines
		}

		fen := strings.TrimSpace(parts[0])
		resultStr := strings.TrimSuffix(strings.TrimSpace(parts[1]), "]")

		resultVal, err := strconv.ParseFloat(resultStr, 64)
		if err != nil {
			continue // skip malformed result
		}

		entry := TEntry{
			fen:    fen,
			index:  linesScanned,
			result: resultVal,
		}

		(*entries)[linesScanned] = entry
		linesScanned++
	}

	if err := sc.Err(); err != nil {
		check(err)
	}
}
