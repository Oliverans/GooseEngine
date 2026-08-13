//go:build tuner_candidate && tuner_seed

package engine

// Build-tag conflict is intentional: a process must choose either the current
// child candidate or the immutable parent seed overlay, never both.
const tunerCandidateAndSeedOverlaysAreMutuallyExclusive = uint(1) / uint(0)
