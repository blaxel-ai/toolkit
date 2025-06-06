import os
from contextlib import asynccontextmanager
from logging import getLogger

from fastapi import FastAPI
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor

from .server.error import init_error_handlers
from .server.middleware import init_middleware
from .server.router import router

logger = getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    logger.info(f"Server running on port {os.getenv('BL_SERVER_PORT', 80)}")
    yield
    logger.info("Server shutting down")

app = FastAPI(lifespan=lifespan)
init_error_handlers(app)
init_middleware(app)
app.include_router(router)


FastAPIInstrumentor.instrument_app(app)

if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=80)
