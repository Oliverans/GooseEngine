//go:build !amd64

package goosemg

// pextBMI2 fallback for non-amd64 builds. The amd64 build gets the hardware
// PEXTQ implementation from pext_bmi2_amd64.s; on every other architecture
// useBMI2PEXT is false and this software path is used. (The companion file
// pext_bmi2_amd64.go is named with an _amd64 suffix and therefore never
// compiles on non-amd64, so this provides the body those builds need.)
func pextBMI2(x, mask uint64) uint64 {
	return pextSoft(x, mask)
}
