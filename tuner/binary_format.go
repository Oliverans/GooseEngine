package tuner

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// BinarySample is a compact fixed-size representation for disk storage
type BinarySample struct {
	// 12 bitboards: 6 white piece types + 6 black piece types
	WhitePawns   uint64
	WhiteKnights uint64
	WhiteBishops uint64
	WhiteRooks   uint64
	WhiteQueens  uint64
	WhiteKings   uint64
	BlackPawns   uint64
	BlackKnights uint64
	BlackBishops uint64
	BlackRooks   uint64
	BlackQueens  uint64
	BlackKings   uint64
	// Metadata
	STM        uint8   // 1 if white to move, 0 if black
	PiecePhase uint16  // cached phase value
	Label      float32 // 0.0, 0.5, 1.0
	_          uint8   // padding to make it 104 bytes
}

const BinarySampleSize = 104 // 12*8 + 1 + 2 + 4 + 1 padding

// ToBinary converts a Sample to binary format
func (s *Sample) ToBinary() BinarySample {
	bs := BinarySample{
		STM:        uint8(s.STM),
		PiecePhase: uint16(s.PiecePhase),
		Label:      float32(s.Label),
	}

	// Convert piece lists to bitboards
	for _, sq := range s.Pieces[0] { // Pawns
		bs.WhitePawns |= 1 << uint(sq)
	}
	for _, sq := range s.Pieces[1] { // Knights
		bs.WhiteKnights |= 1 << uint(sq)
	}
	for _, sq := range s.Pieces[2] { // Bishops
		bs.WhiteBishops |= 1 << uint(sq)
	}
	for _, sq := range s.Pieces[3] { // Rooks
		bs.WhiteRooks |= 1 << uint(sq)
	}
	for _, sq := range s.Pieces[4] { // Queens
		bs.WhiteQueens |= 1 << uint(sq)
	}
	for _, sq := range s.Pieces[5] { // Kings
		bs.WhiteKings |= 1 << uint(sq)
	}

	for _, sq := range s.BP[0] { // Black Pawns
		bs.BlackPawns |= 1 << uint(sq)
	}
	for _, sq := range s.BP[1] { // Black Knights
		bs.BlackKnights |= 1 << uint(sq)
	}
	for _, sq := range s.BP[2] { // Black Bishops
		bs.BlackBishops |= 1 << uint(sq)
	}
	for _, sq := range s.BP[3] { // Black Rooks
		bs.BlackRooks |= 1 << uint(sq)
	}
	for _, sq := range s.BP[4] { // Black Queens
		bs.BlackQueens |= 1 << uint(sq)
	}
	for _, sq := range s.BP[5] { // Black Kings
		bs.BlackKings |= 1 << uint(sq)
	}

	return bs
}

// ToSample converts binary format back to Sample
func (bs *BinarySample) ToSample() Sample {
	s := Sample{
		STM:        int(bs.STM),
		PiecePhase: int(bs.PiecePhase),
		Label:      float64(bs.Label),
	}

	// Convert bitboards back to piece lists
	s.Pieces[0] = bitboardToSquares(bs.WhitePawns)
	s.Pieces[1] = bitboardToSquares(bs.WhiteKnights)
	s.Pieces[2] = bitboardToSquares(bs.WhiteBishops)
	s.Pieces[3] = bitboardToSquares(bs.WhiteRooks)
	s.Pieces[4] = bitboardToSquares(bs.WhiteQueens)
	s.Pieces[5] = bitboardToSquares(bs.WhiteKings)

	s.BP[0] = bitboardToSquares(bs.BlackPawns)
	s.BP[1] = bitboardToSquares(bs.BlackKnights)
	s.BP[2] = bitboardToSquares(bs.BlackBishops)
	s.BP[3] = bitboardToSquares(bs.BlackRooks)
	s.BP[4] = bitboardToSquares(bs.BlackQueens)
	s.BP[5] = bitboardToSquares(bs.BlackKings)

	return s
}

// bitboardToSquares extracts square indices from a bitboard
func bitboardToSquares(bb uint64) []int {
	var squares []int
	for bb != 0 {
		sq := BitScanForward(bb)
		squares = append(squares, sq)
		bb &= bb - 1 // clear lowest bit
	}
	return squares
}

// BitScanForward returns the index of the least significant bit
func BitScanForward(bb uint64) int {
	// Using De Bruijn multiplication for fast bit scanning
	const debruijn64 = 0x03f79d71b4cb0a89
	var index = [64]int{
		0, 47, 1, 56, 48, 27, 2, 60,
		57, 49, 41, 37, 28, 16, 3, 61,
		54, 58, 35, 52, 50, 42, 21, 44,
		38, 32, 29, 23, 17, 11, 4, 62,
		46, 55, 26, 59, 40, 36, 15, 53,
		34, 51, 20, 43, 31, 22, 10, 45,
		25, 39, 14, 33, 19, 30, 9, 24,
		13, 18, 8, 12, 7, 6, 5, 63,
	}
	return index[((bb^(bb-1))*debruijn64)>>58]
}

// WriteBinary writes a binary sample to a writer
func (bs *BinarySample) WriteBinary(w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, bs)
}

// ReadBinary reads a binary sample from a reader
func (bs *BinarySample) ReadBinary(r io.Reader) error {
	return binary.Read(r, binary.LittleEndian, bs)
}

// ConvertToBinary converts a TSV/CSV file to binary format
func ConvertToBinary(tsvPath, binPath string, isCSV bool, maxRows int) error {
	fmt.Printf("Loading dataset from %s...\n", tsvPath)
	samples, err := LoadDataset(tsvPath, isCSV, maxRows)
	if err != nil {
		return fmt.Errorf("load dataset: %w", err)
	}
	fmt.Printf("Loaded %d samples\n", len(samples))

	fmt.Printf("Converting to binary format...\n")
	f, err := os.Create(binPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	// Write header: number of samples
	if err := binary.Write(f, binary.LittleEndian, uint64(len(samples))); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// Write all samples
	for i, s := range samples {
		bs := s.ToBinary()
		if err := bs.WriteBinary(f); err != nil {
			return fmt.Errorf("write sample %d: %w", i, err)
		}
		if (i+1)%100000 == 0 {
			fmt.Printf("  Converted %d/%d samples...\n", i+1, len(samples))
		}
	}

	fmt.Printf("Successfully converted %d samples to %s\n", len(samples), binPath)
	fmt.Printf("Binary file size: %.2f MB\n", float64(8+len(samples)*BinarySampleSize)/(1024*1024))
	fmt.Printf("Compression ratio: %.2fx\n", float64(len(samples)*250)/float64(len(samples)*BinarySampleSize))
	return nil
}

// LoadBinaryDataset loads all samples from a binary file
func LoadBinaryDataset(path string, maxRows int) ([]BinarySample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	// Read header
	var count uint64
	if err := binary.Read(f, binary.LittleEndian, &count); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	if maxRows > 0 && maxRows < int(count) {
		count = uint64(maxRows)
	}

	samples := make([]BinarySample, count)
	for i := uint64(0); i < count; i++ {
		if err := samples[i].ReadBinary(f); err != nil {
			return nil, fmt.Errorf("read sample %d: %w", i, err)
		}
	}

	return samples, nil
}

// LoadBinaryBatch loads a specific batch of samples from a binary file
func LoadBinaryBatch(path string, offset, count int) ([]BinarySample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	// Seek to the batch position (skip header + offset samples)
	seekPos := int64(8 + offset*BinarySampleSize)
	if _, err := f.Seek(seekPos, 0); err != nil {
		return nil, fmt.Errorf("seek to offset %d: %w", offset, err)
	}

	samples := make([]BinarySample, 0, count)
	for i := 0; i < count; i++ {
		var bs BinarySample
		if err := bs.ReadBinary(f); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read sample %d: %w", i, err)
		}
		samples = append(samples, bs)
	}

	return samples, nil
}

// GetBinaryDatasetSize returns the number of samples in a binary file
func GetBinaryDatasetSize(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	var count uint64
	if err := binary.Read(f, binary.LittleEndian, &count); err != nil {
		return 0, fmt.Errorf("read header: %w", err)
	}

	return int(count), nil
}
