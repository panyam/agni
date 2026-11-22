//go:build !windows

package records

// openElsewhere is false off windows, where a rename over or a remove of an open file succeeds.
func openElsewhere(error) bool { return false }
