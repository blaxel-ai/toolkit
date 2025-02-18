"""Module: app

This module sets up and runs the Blaxel server using FastAPI.
It configures middleware, handles server lifespan events, and defines endpoints.
"""

import asyncio
import importlib
import inspect
import os
import sys
import traceback
from contextlib import asynccontextmanager
from logging import getLogger
from uuid import uuid4

from asgi_correlation_id import CorrelationIdMiddleware
from fastapi import FastAPI, Request, Response, WebSocket
from fastapi.responses import JSONResponse

from blaxel.aimon import HTTPError, get_settings, init
from blaxel.aimon.instrumentation import instrument_app, shutdown_instrumentation

from .middlewares import AccessLogMiddleware, AddProcessTimeHeader

sys.path.insert(0, os.getcwd())
sys.path.insert(0, os.path.join(os.getcwd(), "src"))


def import_module():
    """Dynamically imports the main server module based on settings.

    Returns:
        Callable: The main function to run the server.
    """
    settings = get_settings()
    main_module = importlib.import_module(".".join(settings.server.module.split(".")[0:-1]))
    func = getattr(main_module, settings.server.module.split(".")[-1])
    return func


settings = init()
logger = getLogger(__name__)
logger.info(f"Importing server module: {settings.server.module}")
func = import_module()
logger.info(
    f"Running server"
    f" on {settings.server.host}:{settings.server.port}"
)
func_params = inspect.signature(func).parameters
websocket_detected = False
if "websocket" in func_params:
    websocket_detected = True

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manages the lifespan events of the FastAPI application.

    Args:
        app (FastAPI): The FastAPI application instance.
    """
    yield
    shutdown_instrumentation()


app = FastAPI(docs_url=None, redoc_url=None, lifespan=lifespan)
app.add_middleware(
    CorrelationIdMiddleware,
    header_name="x-blaxel-request-id",
    generator=lambda: str(uuid4()),
)
app.add_middleware(AddProcessTimeHeader)
app.add_middleware(AccessLogMiddleware)
instrument_app(app)

@app.get("/health")
async def health():
    """Health check endpoint.

    Returns:
        dict: A simple status message indicating the server is running.
    """
    return {"status": "ok"}

if websocket_detected:
    @app.websocket("/ws")
    async def websocket_endpoint(websocket: WebSocket):
        await websocket.accept()

        original_func = getattr(func, "__wrapped__", func)
        if asyncio.iscoroutinefunction(func) or asyncio.iscoroutinefunction(original_func):
            await func(websocket)
        else:
            func(websocket)

else:
    @app.post("/")
    async def root(request: Request):
        logger = getLogger(__name__)
        try:
            original_func = getattr(func, "__wrapped__", func)
            if asyncio.iscoroutinefunction(func) or asyncio.iscoroutinefunction(original_func):
                response = await func(request)
            else:
                response = func(request)

            if isinstance(response, Response):
                return response
            if type(response) is str:
                return Response(
                    content=response,
                    headers={"Content-Type": "text/plain"},
                    media_type="text/plain",
                    status_code=200,
                )
            return JSONResponse(status_code=200, content=response)
        except ValueError as e:
            content = {"error": str(e)}
            content["traceback"] = str(traceback.format_exc())
            logger.error(str(traceback.format_exc()))
            return JSONResponse(status_code=400, content=content)
        except HTTPError as e:
            content = {"error": e.message, "status_code": e.status_code}
            content["traceback"] = str(traceback.format_exc())
            logger.error(f"{e.status_code} {str(traceback.format_exc())}")
            return JSONResponse(status_code=e.status_code, content=content)
        except Exception as e:
            content = {"error": f"Internal server error, {e}"}
            content["traceback"] = str(traceback.format_exc())
            logger.error(str(traceback.format_exc()))
            return JSONResponse(status_code=500, content=content)
