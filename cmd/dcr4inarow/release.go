//go:build desktop && !dev

package main

import "fmt"

// open refuses the fixture in an ordinary build.
//
// The fixture is unpaid play with no log behind it, and every player-facing
// match is staked. A build a player installs has no route to it at all, rather
// than a flag somebody could find.
func openFixture() (source, error) {
	return nil, fmt.Errorf("-dev-board is not available in this build")
}
