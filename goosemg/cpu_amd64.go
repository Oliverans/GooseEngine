//go:build amd64

package goosemg

// cpuid is implemented in cpu_amd64.s.
//
//go:noescape
func cpuid(eaxArg, ecxArg uint32) (eax, ebx, ecx, edx uint32)

// This file is named to sort before movegen.go so that its init runs first and
// initSliderTables builds its tables through the fast path. Ordering is a
// performance detail only: pextBMI2 and pextSoft compute the same function, so
// the table index scheme is identical either way.
func init() {
	useBMI2PEXT = hasFastPEXT()
}

// hasFastPEXT reports whether this CPU has BMI2 AND implements PEXT in hardware
// rather than microcode.
//
// The second half matters. AMD implemented PEXT in microcode until Zen 3, at
// roughly 18 cycles against Intel's 3 -- slower than the pextSoft loop this flag
// exists to replace, so a bare BMI2 check would be a regression there. AMD is
// therefore only trusted from family 0x19 (Zen 3) onward; Intel is trusted
// whenever it reports BMI2, which means Haswell onward.
//
// No XSAVE/XGETBV check is needed. BMI2 operates on general-purpose registers
// and carries no OS-enabled state, unlike AVX.
func hasFastPEXT() bool {
	maxLeaf, ebx0, ecx0, edx0 := cpuid(0, 0)
	if maxLeaf < 7 {
		return false
	}

	const bmi2Bit = 1 << 8 // CPUID leaf 7, subleaf 0, EBX bit 8
	if _, ebx7, _, _ := cpuid(7, 0); ebx7&bmi2Bit == 0 {
		return false
	}

	// Leaf 0 returns the vendor string in EBX, EDX, ECX in that order.
	const (
		amdEBX = 0x68747541 // "Auth"
		amdEDX = 0x69746e65 // "enti"
		amdECX = 0x444d4163 // "cAMD"
	)
	if ebx0 == amdEBX && edx0 == amdEDX && ecx0 == amdECX {
		eax1, _, _, _ := cpuid(1, 0)
		family := (eax1 >> 8) & 0xf
		if family == 0xf {
			family += (eax1 >> 20) & 0xff
		}
		return family >= 0x19
	}
	return true
}
