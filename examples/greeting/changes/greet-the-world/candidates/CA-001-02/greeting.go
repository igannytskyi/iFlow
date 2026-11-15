// Package greeting says hello.
package greeting

// Greet returns the greeting for the person named, or for the world when
// nobody is.
func Greet(name string) string {
	if name == "" {
		name = "world"
	}
	return "Hello, " + name + "!"
}
