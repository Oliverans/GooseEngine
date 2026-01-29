package engine

// Min returns the smaller of x or y.
func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// Max returns the larger of x or y.
func Max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func Max32(x, y int32) int32 {
	if x > y {
		return x
	}
	return y
}

func Max8(x, y int8) int8 {
	if x > y {
		return x
	}
	return y
}

// abs32 returns the absolute value of x.
func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// Clamp restricts f to the inclusive range [low, high].
func Clamp(f, low, high int8) int8 {
	if f < low {
		return low
	}
	if f > high {
		return high
	}
	return f
}
