//go:build !amd64

package goosemg

func pextBMI2(x, mask uint64) uint64 {
	// Not used; useBMI2PEXT will always be false on non-amd64.
	return pextSoft(x, mask)
}
