"""
This module defines custom exception classes used for handling HTTP-related errors within Blaxel.
"""

class HTTPError(Exception):
    """
    A custom exception class for HTTP errors.

    Attributes:
        status_code (int): The HTTP status code associated with the error.
        message (str): A descriptive message explaining the error.
    """

    def __init__(self, status_code: int, message: str):
        self.status_code = status_code
        self.message = message
        super().__init__(self.message)

    def __str__(self):
        """
        Returns a string representation of the HTTPError.

        Returns:
            str: A string in the format "status_code message".
        """
        return f"{self.status_code} {self.message}"

