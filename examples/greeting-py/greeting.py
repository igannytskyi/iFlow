"""Says hello."""


def greet(name: str) -> str:
    """The greeting for the person named, or for the world when nobody is."""
    name = (name or "").strip()
    if not name:
        name = "world"
    return f"Hello, {name}!"
