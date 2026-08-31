package engine

import (
	"testing"
	"unsafe"
)

func newTestTT() TransTable {
	return TransTable{
		isInitialized: true,
		buckets:       make([]TTBucket, 1),
		size:          1,
		mask:          0,
		generation:    1,
	}
}

func TestTTEntrySize(t *testing.T) {
	if got := unsafe.Sizeof(TTEntry{}); got != 16 {
		t.Fatalf("TTEntry size = %d, want 16", got)
	}
}

func TestTTLockRejectsUpperHashCollision(t *testing.T) {
	tt := newTestTT()
	hashA := uint64(0x12345678)<<32 | uint64(0x11)<<24
	hashB := uint64(0x12345678)<<32 | uint64(0x22)<<24

	tt.storeEntry(hashA, 8, 0, 1, 25, 10, ExactFlag, false)
	if _, found := tt.ProbeEntry(hashA); !found {
		t.Fatal("stored hash was not found")
	}
	if _, found := tt.ProbeEntry(hashB); found {
		t.Fatal("upper-32 collision passed the TT lock")
	}
}

func TestTTFlagEncoding(t *testing.T) {
	if AlphaFlag != 0 || BetaFlag != 1 || ExactFlag != 2 {
		t.Fatalf("unexpected TT bounds: alpha %d beta %d exact %d", AlphaFlag, BetaFlag, ExactFlag)
	}
	for _, bound := range []int8{AlphaFlag, BetaFlag, ExactFlag} {
		plain := makeTTFlag(bound, false)
		if ttBound(plain) != bound || ttPV(plain) {
			t.Fatalf("plain flag %d did not preserve bound %d", plain, bound)
		}
		pv := makeTTFlag(bound, true)
		if ttBound(pv) != bound || !ttPV(pv) {
			t.Fatalf("PV flag %d did not preserve bound %d", pv, bound)
		}
	}

	tt := newTestTT()
	hash := uint64(0x12345678) << 32
	tt.storeEntry(hash, 4, 0, 1, 30, 20, ExactFlag, true)
	entry, found := tt.ProbeEntry(hash)
	usable, score := tt.useEntry(entry, hash, 4, -100, 100, 0, 0)
	if !found || !ttPV(entry.Flag) || !usable || score != 30 {
		t.Fatalf("PV exact entry = (found %v, pv %v, usable %v, score %d)", found, ttPV(entry.Flag), usable, score)
	}
	if tt.scoreEntryForReplacement(entry, 4) <= tt.scoreEntryForReplacement(&TTEntry{Depth: 4, Flag: AlphaFlag}, 4) {
		t.Fatal("PV exact entry lost its exact replacement bonus")
	}
}

func TestNodeTTPv(t *testing.T) {
	if !nodeTTPv(true, false, AlphaFlag) {
		t.Fatal("PV node did not set ttPv")
	}
	if !nodeTTPv(false, true, makeTTFlag(BetaFlag, true)) {
		t.Fatal("probed ttPv was not inherited")
	}
	if nodeTTPv(false, true, BetaFlag) || nodeTTPv(false, false, makeTTFlag(BetaFlag, true)) {
		t.Fatal("ttPv was set without a PV node or matching TT hit")
	}
}

func TestTTScoreRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		score     int32
		storePly  int8
		probePly  int8
		wantScore int32
	}{
		{"positive", 315, 4, 9, 315},
		{"negative", -712, 4, 9, -712},
		{"positive mate", MaxScore - 5, 5, 9, MaxScore - 9},
		{"negative mate", -MaxScore + 5, 5, 9, -MaxScore + 9},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tt := newTestTT()
			hash := uint64(0x12345678+uint32(i)) << 32
			tt.storeEntry(hash, 8, test.storePly, 1, test.score, 20, ExactFlag, false)
			entry, found := tt.ProbeEntry(hash)
			if !found {
				t.Fatal("stored entry was not found")
			}
			usable, score := tt.useEntry(entry, hash, 8, -MaxScore, MaxScore, test.probePly, 0)
			if !usable || score != test.wantScore {
				t.Fatalf("useEntry() = (%v, %d), want (true, %d)", usable, score, test.wantScore)
			}
		})
	}
}

func TestTTBoundCompatibility(t *testing.T) {
	tests := []struct {
		name       string
		bound      int8
		stored     int32
		alpha      int32
		beta       int32
		wantUsable bool
		wantScore  int32
	}{
		{"alpha cutoff", AlphaFlag, -20, -10, 10, true, -10},
		{"alpha miss", AlphaFlag, -20, -30, 10, false, -20},
		{"beta cutoff", BetaFlag, 20, -10, 10, true, 10},
		{"beta miss", BetaFlag, 20, -10, 30, false, 20},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tt := newTestTT()
			hash := uint64(0x22345678+uint32(i)) << 32
			tt.storeEntry(hash, 6, 0, 1, test.stored, 15, test.bound, true)
			entry, _ := tt.ProbeEntry(hash)
			usable, score := tt.useEntry(entry, hash, 6, test.alpha, test.beta, 0, 0)
			if usable != test.wantUsable || score != test.wantScore {
				t.Fatalf("useEntry() = (%v, %d), want (%v, %d)", usable, score, test.wantUsable, test.wantScore)
			}
		})
	}
}

func TestTTStaticEval(t *testing.T) {
	tt := newTestTT()
	hash := uint64(0x12345678) << 32
	tt.storeEntry(hash, 6, 0, 1, 25, 83, ExactFlag, false)
	entry, found := tt.ProbeEntry(hash)
	if eval, ok := ttStaticEval(entry, found); !ok || eval != 83 {
		t.Fatalf("ttStaticEval() = (%d, %v), want (83, true)", eval, ok)
	}

	tt.storeEntry(hash, 6, 0, 1, 25, -MaxScore, ExactFlag, false)
	entry, found = tt.ProbeEntry(hash)
	if eval, ok := ttStaticEval(entry, found); ok {
		t.Fatalf("sentinel static eval returned (%d, true)", eval)
	}
}

func TestTTShallowStoreFillsMissingStaticEval(t *testing.T) {
	tt := newTestTT()
	hash := uint64(0x12345678) << 32
	tt.storeEntry(hash, 8, 0, 1, 120, -MaxScore, ExactFlag, false)
	tt.storeEntry(hash, 0, 0, 2, 30, 44, AlphaFlag, false)

	entry, found := tt.ProbeEntry(hash)
	if !found {
		t.Fatal("stored entry was not found")
	}
	if entry.Depth != 8 || int32(entry.Score) != 120 || ttBound(entry.Flag) != ExactFlag {
		t.Fatalf("deeper entry was replaced: %+v", *entry)
	}
	if eval, ok := ttStaticEval(entry, true); !ok || eval != 44 {
		t.Fatalf("static eval = (%d, %v), want (44, true)", eval, ok)
	}
}

func TestTTShallowStoreSetsTTPv(t *testing.T) {
	tt := newTestTT()
	hash := uint64(0x12345678) << 32
	tt.storeEntry(hash, 8, 0, 1, 120, 30, ExactFlag, false)
	tt.storeEntry(hash, 0, 0, 2, 20, 30, AlphaFlag, true)

	entry, found := tt.ProbeEntry(hash)
	if !found || !ttPV(entry.Flag) || ttBound(entry.Flag) != ExactFlag || entry.Depth != 8 {
		t.Fatalf("shallow ttPv store changed deeper entry incorrectly: %+v", *entry)
	}
}

func TestTTDepthZeroDoesNotSatisfyMainSearch(t *testing.T) {
	tt := newTestTT()
	hash := uint64(0x12345678) << 32
	tt.storeEntry(hash, 0, 0, 1, 50, 25, ExactFlag, false)
	entry, found := tt.ProbeEntry(hash)
	if !found {
		t.Fatal("stored entry was not found")
	}

	if usable, _ := tt.useEntry(entry, hash, 1, -100, 100, 0, 0); usable {
		t.Fatal("depth-0 entry satisfied a depth-1 request")
	}
	if usable, score := tt.useEntry(entry, hash, 0, -100, 100, 0, 0); !usable || score != 50 {
		t.Fatalf("depth-0 request = (%v, %d), want (true, 50)", usable, score)
	}
}
