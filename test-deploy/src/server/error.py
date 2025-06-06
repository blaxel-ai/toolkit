import logging

from fastapi import FastAPI, HTTPException, Request, status
from fastapi.encoders import jsonable_encoder
from fastapi.responses import JSONResponse

logger = logging.getLogger(__name__)

def init_error_handlers(app: FastAPI):
    @app.exception_handler(Exception)
    async def exception_handler(request: Request, e: Exception):
        logger.error(f"Error during request: {e}", exc_info=e)
        return JSONResponse(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            content=jsonable_encoder({"error": str(e)}),
        )

    @app.exception_handler(HTTPException)
    async def http_exception_handler(request: Request, e: HTTPException):
        logger.error(f"Error during request: {e}", exc_info=e)
        return JSONResponse(
            status_code=e.status_code,
            content=jsonable_encoder({"error": str(e)}),
        )
