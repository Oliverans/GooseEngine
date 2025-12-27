// tuner/data.go
package tuner

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func parseLabel(s string) (float64, error) {
	switch s {
	case "1-0":
		return 1.0, nil
	case "0-1":
		return 0.0, nil
	case "1/2-1/2", "1/2", "0.5":
		return 0.5, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if f < 0 || f > 1 {
			return 0, fmt.Errorf("label out of [0,1]: %v", f)
		}
		return f, nil
	}
	return 0, fmt.Errorf("cannot parse label: %q", s)
}

func fenToSample(fen string, label float64) (Sample, error) {
	parts := strings.Split(fen, " ")
	if len(parts) < 2 {
		return Sample{}, fmt.Errorf("bad FEN: %q", fen)
	}
	board, stm := parts[0], parts[1]
	var s Sample
	s.STM = 1
	if stm == "b" {
		s.STM = 0
	}

	// board ranks 8..1
	ranks := strings.Split(board, "/")
	if len(ranks) != 8 {
		return Sample{}, fmt.Errorf("bad board ranks: %q", fen)
	}
	sq := 56 // a8
	for _, r := range ranks {
		file := 0
		for i := 0; i < len(r); i++ {
			ch := r[i]
			if ch >= '1' && ch <= '8' {
				step := int(ch - '0')
				file += step
				sq += step
				continue
			}
			idx, ok := pieceIndex[ch]
			if !ok {
				return Sample{}, fmt.Errorf("bad piece char: %c", ch)
			}
			if ch >= 'a' { // black
				s.BP[idx] = append(s.BP[idx], sq)
			} else {
				s.Pieces[idx] = append(s.Pieces[idx], sq)
			}
			file++
			sq++
		}
		if file != 8 {
			return Sample{}, fmt.Errorf("bad file count in rank")
		}
		sq -= 16
	}
	// cache phase
	s.PiecePhase = countPhase(s)
	s.Label = label
	return s, nil
}

func countPhase(s Sample) int {
	count := func(arr [6][]int, brr [6][]int, idx int) int {
		return len(arr[idx]) + len(brr[idx])
	}
	phase := 0
	phase += count(s.Pieces, s.BP, N) * KnightPhase
	phase += count(s.Pieces, s.BP, B) * BishopPhase
	phase += count(s.Pieces, s.BP, R) * RookPhase
	phase += count(s.Pieces, s.BP, Q) * QueenPhase
	return phase
}

func LoadDataset(path string, isCSV bool, maxRows int) ([]Sample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var r *csv.Reader
	if isCSV {
		r = csv.NewReader(bufio.NewReader(f))
		r.Comma = ','
	} else {
		r = csv.NewReader(bufio.NewReader(f))
		r.Comma = '\t'
	}
	r.FieldsPerRecord = -1

	var out []Sample
	line := 0
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read error line %d: %w", line, err)
		}
		line++
		var fen, lab string
		if len(rec) >= 2 {
			fen = strings.TrimSpace(rec[0])
			lab = strings.TrimSpace(rec[1])
		} else if len(rec) == 1 {
			// Fallback: single-field line like "<FEN> [0.5]"
			raw := strings.TrimSpace(rec[0])
			// Try to find trailing bracketed label
			li := strings.LastIndex(raw, "[")
			rj := strings.LastIndex(raw, "]")
			if li >= 0 && rj > li {
				fen = strings.TrimSpace(raw[:li])
				lab = strings.TrimSpace(raw[li+1 : rj])
			} else {
				// As a last resort, split on whitespace and take last token as label
				parts := strings.Fields(raw)
				if len(parts) >= 7 { // full FEN has 6 fields; 7th may be label
					fen = strings.Join(parts[:6], " ")
					lab = parts[len(parts)-1]
				} else {
					continue
				}
			}
		} else {
			continue
		}
		y, err := parseLabel(lab)
		if err != nil {
			continue
		}
		s, err := fenToSample(fen, y)
		if err != nil {
			continue
		}
		out = append(out, s)
		if maxRows > 0 && len(out) >= maxRows {
			break
		}
	}
	return out, nil
}
