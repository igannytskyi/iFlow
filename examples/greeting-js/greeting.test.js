const test = require("node:test");
const assert = require("node:assert");
const { greet } = require("./greeting.js");

test("greet names the person", () => {
  for (const name of ["Ada", "Linus", "Grace Hopper"]) {
    assert.strictEqual(greet(name), "Hello, " + name + "!");
  }
});

test("greet greets the world when nobody is named", () => {
  assert.strictEqual(greet(""), "Hello, world!");
  assert.strictEqual(greet("   "), "Hello, world!");
});
