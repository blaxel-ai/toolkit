"""Module: processtime

Defines the AddProcessTimeHeader middleware for adding process time information to responses.
"""

import time

from starlette.middleware.base import BaseHTTPMiddleware


class AddProcessTimeHeader(BaseHTTPMiddleware):
    """Middleware to add the X-Process-Time header to each HTTP response."""

    async def dispatch(self, request, call_next):
        """Calculates and adds the processing time to the response headers.

        Args:
            request (Request): The incoming HTTP request.
            call_next (Callable): The next middleware or endpoint handler.

        Returns:
            Response: The HTTP response with the X-Process-Time header added.
        """
        start_time = time.perf_counter()
        response = await call_next(request)
        process_time = (time.perf_counter() - start_time) * 1000
        response.headers["X-Process-Time"] = f"{process_time:.2f}"
        return response
