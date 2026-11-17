import unittest

from greeting import greet


class GreetTest(unittest.TestCase):
    def test_greet_names_the_person(self):
        for name in ("Ada", "Linus", "Grace Hopper"):
            self.assertEqual(greet(name), f"Hello, {name}!")

    def test_greet_greets_the_world_when_nobody_is_named(self):
        self.assertEqual(greet(""), "Hello, world!")
        self.assertEqual(greet("   "), "Hello, world!")


if __name__ == "__main__":
    unittest.main()
