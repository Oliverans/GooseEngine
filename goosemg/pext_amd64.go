//go:build amd64

package goosemg

// pextBMI2 is provided in pext_bmi2_amd64.s on amd64 and uses the BMI2 PEXTQ
// instruction. The software fallback for other architectures lives in
// pext_fallback.go.
//
//go:noescape
func pextBMI2(x, mask uint64) uint64
