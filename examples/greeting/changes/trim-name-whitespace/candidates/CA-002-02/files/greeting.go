// Package greeting says hello.
package greeting

import "strings"

// Greet returns the greeting for the person named, or for the world when
// nobody is.
func Greet(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "world"
	}
	return "Hello, " + name + "!"
}
