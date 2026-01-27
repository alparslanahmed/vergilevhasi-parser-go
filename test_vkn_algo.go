package main

import (
	"fmt"
)

// Test with different VKN algorithms
func main() {
	// Real VKNs from test PDFs
	testVKNs := []string{
		"1222153986",
		"8589706200",
		"1234567890",
	}

	for _, vkn := range testVKNs {
		fmt.Printf("\nTesting VKN: %s\n", vkn)
		fmt.Printf("  Algorithm 1 (standard): %v\n", isValidVKN1(vkn))
		fmt.Printf("  Algorithm 2 (alternate): %v\n", isValidVKN2(vkn))
		fmt.Printf("  Algorithm 3 (simple): %v\n", isValidVKN3(vkn))
	}
}

// Standard algorithm
func isValidVKN1(vkn string) bool {
	if len(vkn) != 10 {
		return false
	}

	digits := make([]int, 10)
	for i, ch := range vkn {
		if ch < '0' || ch > '9' {
			return false
		}
		digits[i] = int(ch - '0')
	}

	sum := 0
	for i := 0; i < 9; i++ {
		v1 := (digits[i] + 10 - (i + 1)) % 10
		var v2 int
		if v1 == 0 {
			v2 = 9
		} else {
			power := 9 - i
			temp := v1
			for j := 0; j < power; j++ {
				temp = (temp * 2) % 9
			}
			v2 = temp
			if v2 == 0 {
				v2 = 9
			}
		}
		sum += v2
	}

	expected := (10 - (sum % 10)) % 10
	fmt.Printf("    Sum=%d, Expected=%d, Got=%d\n", sum, expected, digits[9])
	return digits[9] == expected
}

// Alternate algorithm from GIB
func isValidVKN2(vkn string) bool {
	if len(vkn) != 10 {
		return false
	}

	digits := make([]int, 10)
	for i, ch := range vkn {
		if ch < '0' || ch > '9' {
			return false
		}
		digits[i] = int(ch - '0')
	}

	v := make([]int, 9)
	lastDigitTotal := 0

	for i := 0; i < 9; i++ {
		tmp := (digits[i] + (9 - i)) % 10
		if tmp == 9 {
			v[i] = 9
		} else {
			power := intPow(2, 9-i) % 9
			v[i] = (tmp * power) % 9
			if v[i] == 0 && tmp != 0 {
				v[i] = 9
			}
		}
		lastDigitTotal += v[i]
	}

	expected := (10 - (lastDigitTotal % 10)) % 10
	fmt.Printf("    Total=%d, Expected=%d, Got=%d\n", lastDigitTotal, expected, digits[9])
	return digits[9] == expected
}

// Simple algorithm - just check structure
func isValidVKN3(vkn string) bool {
	if len(vkn) != 10 {
		return false
	}
	if vkn[0] == '0' {
		return false
	}
	for _, ch := range vkn {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func intPow(base, n int) int {
	result := 1
	for i := 0; i < n; i++ {
		result *= base
	}
	return result
}
