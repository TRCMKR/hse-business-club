//go:build race

package domain_test

// raceEnabled relaxes timing assertions: the race detector slows decoding ~30x.
const raceEnabled = true
