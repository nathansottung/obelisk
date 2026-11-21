//go:build !unix && !guionly

package main

import "errors"

func runAsNobody(string, ...string) error {
	return errors.New("no second account on this platform")
}
