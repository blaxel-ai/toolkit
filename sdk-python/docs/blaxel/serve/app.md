Module blaxel.serve.app
=======================
Module: app

This module sets up and runs the Blaxel server using FastAPI.
It configures middleware, handles server lifespan events, and defines endpoints.

Functions
---------

`health()`
:   Health check endpoint.
    
    Returns:
        dict: A simple status message indicating the server is running.

`import_module()`
:   Dynamically imports the main server module based on settings.
    
    Returns:
        Callable: The main function to run the server.

`lifespan(app: fastapi.applications.FastAPI)`
:   Manages the lifespan events of the FastAPI application.
    
    Args:
        app (FastAPI): The FastAPI application instance.

`root(request: starlette.requests.Request)`
: