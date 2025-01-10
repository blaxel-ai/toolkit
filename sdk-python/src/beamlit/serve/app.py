import asyncio
import importlib
import os
import sys
import traceback
from logging import getLogger
from uuid import uuid4

from asgi_correlation_id import CorrelationIdMiddleware
from fastapi import FastAPI, Request, Response
from fastapi.responses import JSONResponse
from traceloop.sdk import Traceloop

from beamlit.common import HTTPError, get_settings, init
from beamlit.common.instrumentation import (
    get_metrics_exporter,
    get_resource_attributes,
    get_span_exporter,
    instrument_app,
)

from .middlewares import AccessLogMiddleware, AddProcessTimeHeader

sys.path.insert(0, os.getcwd())
sys.path.insert(0, os.path.join(os.getcwd(), "src"))


def import_module():
    settings = get_settings()
    main_module = importlib.import_module(".".join(settings.server.module.split(".")[0:-1]))
    func = getattr(main_module, settings.server.module.split(".")[-1])
    return func


settings = init()
logger = getLogger(__name__)
logger.info(f"Importing server module: {settings.server.module}")
func = import_module()
logger.info(
    f"Running server with environment {settings.environment}"
    f" on {settings.server.host}:{settings.server.port}"
)

if settings.enable_opentelemetry:
    Traceloop.init(
        app_name=settings.name,
        exporter=get_span_exporter(),
        metrics_exporter=get_metrics_exporter(),
        resource_attributes=get_resource_attributes(),
        should_enrich_metrics=os.getenv("ENRICHED_METRICS", "false") == "true",
    )

app = FastAPI(docs_url=None, redoc_url=None)
app.add_middleware(
    CorrelationIdMiddleware,
    header_name="x-beamlit-request-id",
    generator=lambda: str(uuid4()),
)
app.add_middleware(AddProcessTimeHeader)
app.add_middleware(AccessLogMiddleware)
instrument_app(app)


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.post("/")
async def root(request: Request):
    settings = get_settings()
    logger = getLogger(__name__)
    try:
        body = await request.json()

        original_func = getattr(func, "__wrapped__", func)
        if asyncio.iscoroutinefunction(func) or asyncio.iscoroutinefunction(original_func):
            response = await func(body)
        else:
            response = func(body)

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
        if settings.environment == "development":
            content["traceback"] = str(traceback.format_exc())
        logger.error(str(traceback.format_exc()))
        return JSONResponse(status_code=400, content=content)
    except HTTPError as e:
        content = {"error": e.message, "status_code": e.status_code}
        if settings.environment == "development":
            content["traceback"] = str(traceback.format_exc())
        logger.error(f"{e.status_code} {str(traceback.format_exc())}")
        return JSONResponse(status_code=e.status_code, content=content)
    except Exception as e:
        content = {"error": f"Internal server error, {e}"}
        if settings.environment == "development":
            content["traceback"] = str(traceback.format_exc())
        logger.error(str(traceback.format_exc()))
        return JSONResponse(status_code=500, content=content)
