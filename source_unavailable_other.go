//go:build !windows

package main

// sourceUnavailable: on these systems an absent mount reads as "does not exist".
func sourceUnavailable(error) bool { return false }

func sourceNetworkError(error) bool { return false }
