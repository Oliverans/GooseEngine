package engine

import "fmt"

// CutStatistics collects counts for each pruning/cutoff mechanism.
type CutStatistics struct {
	TTCutoffs         uint64
	NullMoveCutoffs   uint64
	StaticNullCutoffs uint64
	RazoringCutoffs   uint64
	FutilityPrunes    uint64
	LateMovePrunes    uint64
	BetaCutoffs       uint64
	QStandPatCutoffs  uint64
	QBetaCutoffs      uint64
}

var cutStats CutStatistics

// PrintCutStats controls whether the engine dumps the cut statistics once the
// current search finishes. Set via a CLI/command toggle.
var PrintCutStats bool

func resetCutStats() {
	cutStats = CutStatistics{}
}

func dumpCutStats() {
	fmt.Println("info string Cut statistics:")
	fmt.Printf("info string   TT cutoffs: %d\n", cutStats.TTCutoffs)
	fmt.Printf("info string   Null-move cutoffs: %d\n", cutStats.NullMoveCutoffs)
	fmt.Printf("info string   Static null cutoffs: %d\n", cutStats.StaticNullCutoffs)
	fmt.Printf("info string   Razoring cutoffs: %d\n", cutStats.RazoringCutoffs)
	fmt.Printf("info string   Futility prunes: %d\n", cutStats.FutilityPrunes)
	fmt.Printf("info string   Late move prunes: %d\n", cutStats.LateMovePrunes)
	fmt.Printf("info string   Beta cutoffs: %d\n", cutStats.BetaCutoffs)
	fmt.Printf("info string   QStandPat cutoffs: %d\n", cutStats.QStandPatCutoffs)
	fmt.Printf("info string   QBeta cutoffs: %d\n", cutStats.QBetaCutoffs)
}
