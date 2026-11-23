// Says hello.

// greet returns the greeting for the person named, or for the world when
// nobody is.
function greet(name) {
  name = (name ?? "").trim();
  if (name === "") {
    name = "world";
  }
  return "Hello, " + name + "!";
}

module.exports = { greet };
