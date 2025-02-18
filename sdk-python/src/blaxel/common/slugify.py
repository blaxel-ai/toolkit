"""
This module provides utility functions for string manipulation, including slugification.
The `slugify` function transforms a given string into a URL-friendly slug by replacing spaces and underscores with hyphens and converting the string to lowercase.
"""

def slugify(name: str) -> str:
    """
    Converts a given string into a URL-friendly slug.

    This function transforms the input string by converting it to lowercase and replacing spaces and underscores with hyphens.

    Parameters:
        name (str): The string to slugify.

    Returns:
        str: The slugified version of the input string.
    """
    return name.lower().replace(" ", "-").replace("_", "-")