package engine

import (
	"testing"
	"unsafe"
)

func TestTTEntrySize(t *testing.T) {
	if got := unsafe.Sizeof(TTEntry{}); got != 16 {
		t.Fatalf("TTEntry size = %d, want 16", got)
	}
}

func TestTTLockRejectsUpperHashCollision(t *testing.T) {
	tt := TransTable{
		isInitialized: true,
		buckets:       make([]TTBucket, 1),
		size:          1,
		mask:          0,
	}
	hashA := uint64(0x12345678)<<32 | uint64(0x11)<<24
	hashB := uint64(0x12345678)<<32 | uint64(0x22)<<24

	tt.storeEntry(hashA, 8, 0, 1, 25, ExactFlag)
	if _, found := tt.ProbeEntry(hashA); !found {
		t.Fatal("stored hash was not found")
	}
	if _, found := tt.ProbeEntry(hashB); found {
		t.Fatal("upper-32 collision passed the TT lock")
	}
}
