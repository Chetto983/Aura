//go:build race

package docfields

// raceSlowdown widens a timing budget under the race detector, which runs code several times
// slower than it ships.
const raceSlowdown = 10
