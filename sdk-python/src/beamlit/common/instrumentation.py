import base64
from datetime import datetime
import logging
from typing import Any

from authlib.integrations.requests_client import OAuth2Session
from fastapi import FastAPI
from opentelemetry import _logs, metrics, trace
from opentelemetry._logs import set_logger_provider
from opentelemetry.exporter.otlp.proto.grpc._log_exporter import (
    OTLPLogExporter,
)
from opentelemetry.exporter.otlp.proto.grpc.metric_exporter import (
    OTLPMetricExporter,
)
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import (
    OTLPSpanExporter,
)
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor
from opentelemetry.metrics import NoOpMeterProvider
from opentelemetry.sdk._logs import LoggerProvider, LoggingHandler
from opentelemetry.sdk._logs.export import BatchLogRecordProcessor
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.trace import NoOpTracerProvider
from typing_extensions import Dict

from .settings import get_settings

tracer: trace.Tracer | None = None
meter: metrics.Meter | None = None
logger: LoggerProvider | None = None


oauth_session: OAuth2Session | None = None
current_token: Dict[str, Any] | None = None


def get_token() -> Dict[str, Any]:
    # Fetch settings from the environment or config
    settings = get_settings()

    # Retrieve the base64-encoded credentials from the settings
    base64_creds = (
        settings.authentication.client.credentials
        if settings and settings.authentication.client.credentials
        else ""
    )
    client_id, client_secret = "", ""
    if base64_creds:
        try:
            decoded_creds = base64.b64decode(base64_creds).decode("utf-8")
            client_id, client_secret = decoded_creds.split(":")
        except Exception:
            # Return an empty token if the credentials are invalid with expiration time time.time() + 60
            return {
                "access_token": "",
                "expires_at": datetime.now().timestamp() + 7200,
            }

    else:
        return {
            "access_token": "",
            "expires_at": datetime.now().timestamp() + 7200,
        }

    global oauth_session
    if oauth_session is None:
        oauth_session = OAuth2Session(
            client_id=client_id, client_secret=client_secret
        )

    token = oauth_session.fetch_token(
        url=f"{settings.base_url}/oauth/token",
        client_id=client_id,
        client_secret=client_secret,
    )
    return token


def renew_token_if_needed() -> Dict[str, Any]:
    global current_token
    if (
        current_token is None
        or current_token["expires_at"] < datetime.now().timestamp()
    ):
        current_token = get_token()
    return current_token


def get_auth_headers() -> Dict[str, str]:
    token = renew_token_if_needed()
    return {
        "authorization": f"Bearer {token['access_token']}",
        "x-beamlit-workspace": get_settings().workspace,
    }


def get_logger() -> LoggerProvider:
    if logger is None:
        raise Exception("Logger is not initialized")
    return logger


def get_resource_attributes() -> Dict[str, Any]:
    resources = Resource.create()
    resources_dict: Dict[str, Any] = {}
    for key in resources.attributes:
        resources_dict[key] = resources.attributes[key]
    settings = get_settings()
    if settings is None:
        raise Exception("Settings are not initialized")
    resources_dict["workspace"] = settings.workspace
    resources_dict["service.name"] = settings.name
    return resources_dict


def get_metrics_exporter() -> OTLPMetricExporter | None:
    settings = get_settings()
    if not settings.enable_opentelemetry:
        return None
    return OTLPMetricExporter(headers=get_auth_headers())


def get_span_exporter() -> OTLPSpanExporter | None:
    settings = get_settings()
    if not settings.enable_opentelemetry:
        return None
    return OTLPSpanExporter(headers=get_auth_headers())


def get_log_exporter() -> OTLPLogExporter | None:
    settings = get_settings()
    if not settings.enable_opentelemetry:
        return None
    return OTLPLogExporter(headers=get_auth_headers())


def instrument_app(app: FastAPI):
    global tracer
    global meter
    settings = get_settings()
    if not settings.enable_opentelemetry:
        # Use NoOp implementations to stub tracing and metrics
        trace.set_tracer_provider(NoOpTracerProvider())
        tracer = trace.get_tracer(__name__)

        metrics.set_meter_provider(NoOpMeterProvider())
        meter = metrics.get_meter(__name__)
        return

    resource = Resource.create(
        {
            "service.name": settings.name,
            "service.namespace": settings.workspace,
            "service.workspace": settings.workspace,
        }
    )

    # Set up the TracerProvider if not already set
    if not isinstance(trace.get_tracer_provider(), TracerProvider):
        trace_provider = TracerProvider(resource=resource)
        span_processor = BatchSpanProcessor(get_span_exporter())
        trace_provider.add_span_processor(span_processor)
        trace.set_tracer_provider(trace_provider)
        tracer = trace_provider.get_tracer(__name__)
    else:
        tracer = trace.get_tracer(__name__)

    # Set up the MeterProvider if not already set
    if not isinstance(metrics.get_meter_provider(), MeterProvider):
        metrics_exporter = PeriodicExportingMetricReader(get_metrics_exporter())
        meter_provider = MeterProvider(
            resource=resource, metric_readers=[metrics_exporter]
        )
        metrics.set_meter_provider(meter_provider)
        meter = meter_provider.get_meter(__name__)
    else:
        meter = metrics.get_meter(__name__)


    if not isinstance(_logs.get_logger_provider(), LoggerProvider):
        logger_provider = LoggerProvider()
        set_logger_provider(logger_provider)
        logger_provider.add_log_record_processor(
            BatchLogRecordProcessor(get_log_exporter())
        )
        handler = LoggingHandler(
            level=logging.NOTSET, logger_provider=logger_provider
        )
        logging.getLogger().addHandler(handler)
    else:
        logger_provider = _logs.get_logger_provider()

    # Only instrument the app when OpenTelemetry is enabled
    FastAPIInstrumentor.instrument_app(app)
    HTTPXClientInstrumentor().instrument()


def shutdown_instrumentation():
    if tracer is not None:
        trace_provider = trace.get_tracer_provider()
        if isinstance(trace_provider, TracerProvider):
            trace_provider.shutdown()
    if meter is not None:
        meter_provider = metrics.get_meter_provider()
        if isinstance(meter_provider, MeterProvider):
            meter_provider.shutdown()