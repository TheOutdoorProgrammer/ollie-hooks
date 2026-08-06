// Package betterleakstest holds fixtures shared by the secret rules' tests, so
// a token-shaped value valid for nothing is defined once, not per rule.
package betterleakstest

import "strings"

// FakePAT builds a token-shaped value valid for nothing, assembled from
// fragments so no whole token literal sits in source to trip a secret scanner.
func FakePAT() string {
	return strings.Join([]string{"ghp", "_A1b2C3d4E5f6", "G7h8I9j0K1l2", "M3n4O5p6Q7r8"}, "")
}
