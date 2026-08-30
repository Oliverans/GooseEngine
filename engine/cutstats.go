package engine

import (
	"fmt"
	"time"
)

// CutStatistics collects counts for each pruning/cutoff mechanism.
//
// Every field is incremented on the search's hot path. Measured at ~0.15 ns per
// increment against a ~1.2 us node, so the whole set costs well under 0.2% of
// search time. That holds only while the search is single-threaded: under lazy
// SMP these would have to become per-thread or the shared cache line would
// ping-pong between cores.
//
// Attempt/outcome pairs are the point of most of these. A raw cutoff count says
// nothing about whether a mechanism is selective or simply rarely reached.
type CutStatistics struct {
	// Quiescence nodes. Main-search nodes are derived as nodesChecked - QNodes.
	QNodes uint64

	// Main-search transposition table.
	TTProbes  uint64
	TTHits    uint64
	TTUsable  uint64
	TTCutoffs uint64

	// Quiescence transposition table.
	QTTProbes  uint64
	QTTHits    uint64
	QTTCutoffs uint64

	// Reverse futility pruning (static null).
	RFPEligible       uint64
	RFPRefinements    uint64
	RFPEnabledByTT    uint64
	RFPSuppressedByTT uint64
	RFPCutoffs        uint64

	// Null-move pruning.
	NullMoveGateChecks     uint64
	NullMoveRefinements    uint64
	NullMoveEnabledByTT    uint64
	NullMoveSuppressedByTT uint64
	NullMoveAttempts       uint64
	NullMoveCutoffs        uint64

	// Razoring. Attempts count the confirming qsearch calls, so the difference
	// against cutoffs is the verification failure rate.
	RazoringAttempts uint64
	RazoringCutoffs  uint64

	// ProbCut. VerifyFails counts moves whose qsearch cleared the raised beta
	// but whose reduced search then did not.
	ProbCutAttempts      uint64
	ProbCutSeeSkips      uint64
	ProbCutMovesSearched uint64
	ProbCutVerifyFails   uint64
	ProbCutCutoffs       uint64

	// Singular extensions and IID. Both run a full extra search per attempt, so
	// the hit rate is what justifies the cost.
	SingularAttempts uint64
	SingularHits     uint64
	IIDCalls         uint64

	// Late move reductions. The re-search rate is the tuning signal: too high
	// and the reductions are too aggressive, too low and they are too timid.
	LMRReduced    uint64
	LMRResearched uint64

	// Main move loop.
	MovesGenerated            uint64
	MovesSearched             uint64
	MakeMoveRejects           uint64
	FutilityPrunes            uint64
	LateMovePrunes            uint64
	SEENoisyAttempts          uint64
	SEENoisyPrunes            uint64
	SEEQuietAttempts          uint64
	SEEQuietPrunes            uint64
	SEEQuietPriorityProtected uint64

	// Beta cutoffs, bucketed by how many legal moves had been searched when the
	// cutoff landed. Index 0 is the first move, index 3 is the fourth or later.
	// A healthy ordering puts the large majority in bucket 0.
	BetaCutoffs      uint64
	BetaCutoffByMove [4]uint64

	// Quiescence cutoffs.
	QStandPatCutoffs uint64
	QBetaCutoffs     uint64

	// Aspiration windows, counted once per root re-search.
	AspirationFailHigh uint64
	AspirationFailLow  uint64
}

// PrintCutStats controls whether the engine dumps the cut statistics once the
// current search finishes. Set via a CLI/command toggle.
var PrintCutStats bool

func ResetCutStats() {
	SearchState.cutStats = CutStatistics{}
}

// pct renders part as a percentage of whole, guarding the empty denominator so
// an unexercised mechanism reads as "n/a" rather than a misleading 0.0%.
func pct(part, whole uint64) string {
	if whole == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", float64(part)*100/float64(whole))
}

func dumpCutStats() {
	c := SearchState.cutStats
	total := uint64(SearchState.nodesChecked)
	mainNodes := total - c.QNodes

	fmt.Println("info string Search statistics:")
	fmt.Printf("info string   Nodes: %d total, %d main, %d quiescence (%s)\n",
		total, mainNodes, c.QNodes, pct(c.QNodes, total))
	fmt.Printf("info string   TT: %d probes, %d hits (%s), %d usable (%s of hits), %d cutoffs (%s of usable)\n",
		c.TTProbes, c.TTHits, pct(c.TTHits, c.TTProbes),
		c.TTUsable, pct(c.TTUsable, c.TTHits),
		c.TTCutoffs, pct(c.TTCutoffs, c.TTUsable))
	fmt.Printf("info string   QTT: %d probes, %d hits (%s), %d cutoffs (%s of hits)\n",
		c.QTTProbes, c.QTTHits, pct(c.QTTHits, c.QTTProbes),
		c.QTTCutoffs, pct(c.QTTCutoffs, c.QTTHits))
	fmt.Printf("info string   RFP: %d eligible, %d TT refinements (%s), %d enabled, %d suppressed, %d cutoffs (%s)\n",
		c.RFPEligible, c.RFPRefinements, pct(c.RFPRefinements, c.RFPEligible),
		c.RFPEnabledByTT, c.RFPSuppressedByTT,
		c.RFPCutoffs, pct(c.RFPCutoffs, c.RFPEligible))
	fmt.Printf("info string   Null move: %d gate checks, %d TT refinements (%s), %d enabled, %d suppressed, %d attempts, %d cutoffs (%s)\n",
		c.NullMoveGateChecks, c.NullMoveRefinements, pct(c.NullMoveRefinements, c.NullMoveGateChecks),
		c.NullMoveEnabledByTT, c.NullMoveSuppressedByTT, c.NullMoveAttempts,
		c.NullMoveCutoffs, pct(c.NullMoveCutoffs, c.NullMoveAttempts))
	fmt.Printf("info string   Razoring: %d attempts, %d cutoffs (%s)\n",
		c.RazoringAttempts, c.RazoringCutoffs, pct(c.RazoringCutoffs, c.RazoringAttempts))
	fmt.Printf("info string   ProbCut: %d nodes, %d moves searched, %d SEE skips, %d verify fails, %d cutoffs (%s of nodes)\n",
		c.ProbCutAttempts, c.ProbCutMovesSearched, c.ProbCutSeeSkips,
		c.ProbCutVerifyFails, c.ProbCutCutoffs, pct(c.ProbCutCutoffs, c.ProbCutAttempts))
	fmt.Printf("info string   Singular: %d attempts, %d extensions (%s)\n",
		c.SingularAttempts, c.SingularHits, pct(c.SingularHits, c.SingularAttempts))
	fmt.Printf("info string   IID: %d calls\n", c.IIDCalls)
	fmt.Printf("info string   LMR: %d reduced, %d re-searched (%s)\n",
		c.LMRReduced, c.LMRResearched, pct(c.LMRResearched, c.LMRReduced))
	fmt.Printf("info string   Moves: %d generated, %d searched (%s), %d make rejects\n",
		c.MovesGenerated, c.MovesSearched, pct(c.MovesSearched, c.MovesGenerated), c.MakeMoveRejects)
	fmt.Printf("info string   Pruned: %d futility, %d late move\n", c.FutilityPrunes, c.LateMovePrunes)
	fmt.Printf("info string   SEE pruning: noisy %d attempts, %d pruned (%s); quiet %d attempts, %d pruned (%s), %d priority protected\n",
		c.SEENoisyAttempts, c.SEENoisyPrunes, pct(c.SEENoisyPrunes, c.SEENoisyAttempts),
		c.SEEQuietAttempts, c.SEEQuietPrunes, pct(c.SEEQuietPrunes, c.SEEQuietAttempts), c.SEEQuietPriorityProtected)
	fmt.Printf("info string   Beta cutoffs: %d total | move 1: %d (%s), move 2: %d (%s), move 3: %d (%s), move 4+: %d (%s)\n",
		c.BetaCutoffs,
		c.BetaCutoffByMove[0], pct(c.BetaCutoffByMove[0], c.BetaCutoffs),
		c.BetaCutoffByMove[1], pct(c.BetaCutoffByMove[1], c.BetaCutoffs),
		c.BetaCutoffByMove[2], pct(c.BetaCutoffByMove[2], c.BetaCutoffs),
		c.BetaCutoffByMove[3], pct(c.BetaCutoffByMove[3], c.BetaCutoffs))
	fmt.Printf("info string   Quiescence: %d stand-pat cutoffs, %d beta cutoffs\n",
		c.QStandPatCutoffs, c.QBetaCutoffs)
	fmt.Printf("info string   Aspiration: %d fail high, %d fail low\n",
		c.AspirationFailHigh, c.AspirationFailLow)
	th := &SearchState.timeHandler
	if th.isInitialized {
		fmt.Printf("info string   Time: %d ms base, %.0f%% opening, %d ms optimum, %d ms target, %d ms maximum\n",
			th.baseAllocationMillis, th.openingScale*100, th.optimumMillis, th.targetMillis, th.maximumMillis)
		fmt.Printf("info string   Time state: %d ms elapsed, move stable %d depths, score drop %d cp, stop %s\n",
			time.Since(th.startTime).Milliseconds(), th.bestMoveStability, th.lastScoreDrop, th.stopReason)
	}
}
