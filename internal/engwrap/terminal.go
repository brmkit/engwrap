package engwrap

import (
	"golang.org/x/term"
)

const (
	ColorRed    = "\033[91m"
	ColorGreen  = "\033[92m"
	ColorYellow = "\033[93m"
	ColorReset  = "\033[0m"
)

// makeRaw sets terminal to raw mode - cross-platform
// returns the original terminal state that can be used with restoreTerminal
func makeRaw(fd uintptr) (*term.State, error) {
	// convert uintptr to int for golang.org/x/term API
	fdInt := int(fd)

	// check if it's a terminal
	if !term.IsTerminal(fdInt) {
		return nil, nil
	}

	// set terminal to raw mode and save original state
	oldState, err := term.MakeRaw(fdInt)
	if err != nil {
		return nil, err
	}

	return oldState, nil
}

// restore the original terminal state
func restoreTerminal(fd uintptr, state *term.State) error {
	if state == nil {
		return nil
	}

	// convert uintptr to int for golang.org/x/term API
	fdInt := int(fd)
	return term.Restore(fdInt, state)
}

// check if the file descriptor is a terminal - cross-platform
func isTerminal(fd uintptr) bool {
	fdInt := int(fd)
	return term.IsTerminal(fdInt)
}
