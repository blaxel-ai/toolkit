import logging
from time import time

from asgi_correlation_id import CorrelationIdMiddleware
from fastapi import FastAPI, Request, Response
from fastapi.responses import StreamingResponse

logger = logging.getLogger(__name__)

def init_middleware(app: FastAPI):
    app.add_middleware(CorrelationIdMiddleware)

    @app.middleware("http")
    async def log_requests(request: Request, call_next):
        start_time = time()

        response: Response = await call_next(request)

        process_time = (time() - start_time) * 1000
        formatted_process_time = '{0:.2f}'.format(process_time)
        rid_header = response.headers.get("X-Request-Id")
        request_id = rid_header or response.headers.get("X-Blaxel-Request-Id")
        if response.status_code >= 400:
            logger.error(f"{request.method} {request.url.path} {response.status_code} {formatted_process_time}ms rid={request_id}")
        else:
            logger.info(f"{request.method} {request.url.path} {response.status_code} {formatted_process_time}ms rid={request_id}")

        return response