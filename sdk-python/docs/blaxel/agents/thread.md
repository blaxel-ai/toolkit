Module blaxel.agents.thread
===========================
Module: thread

Defines threading capabilities for agents.

Functions
---------

`get_default_thread(request: starlette.requests.Request) ‑> str`
:   Extracts the default thread identifier from an incoming HTTP request.
    Prioritizes the `X-Blaxel-Sub` header and falls back to decoding the JWT
    from the `Authorization` or `X-Blaxel-Authorization` headers.
    
    Parameters:
        request (Request): The incoming HTTP request object.
    
    Returns:
        str: The extracted thread identifier. Returns an empty string if no valid identifier is found.