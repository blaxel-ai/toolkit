def slugify(name: str) -> str:
    return name.lower().replace(" ", "-").replace("_", "-")