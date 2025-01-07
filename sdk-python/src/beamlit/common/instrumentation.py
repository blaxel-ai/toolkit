from typing import Any

from fastapi import FastAPI
from opentelemetry import metrics, trace
from opentelemetry.exporter.otlp.proto.grpc.metric_exporter import (
    OTLPMetricExporter,
)
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import (
    OTLPSpanExporter,
)
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor
from opentelemetry.instrumentation.logging import LoggingInstrumentor
from opentelemetry.metrics import NoOpMeterProvider
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


def get_tracer() -> trace.Tracer:
    if tracer is None:
        raise Exception("Tracer is not initialized")
    return tracer


def get_meter() -> metrics.Meter:
    if meter is None:
        raise Exception("Meter is not initialized")
    return meter


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
    if settings is None:
        raise Exception("Settings are not initialized")
    if not settings.enable_opentelemetry:
        # Return None or a NoOpExporter equivalent
        return None
    return OTLPMetricExporter()


def get_span_exporter() -> OTLPSpanExporter | None:
    settings = get_settings()
    if not settings.enable_opentelemetry:
        return None
    return OTLPSpanExporter()


def instrument_app(app: FastAPI):
    global tracer
    global meter
    settings = get_settings()
    if settings is None:
        raise Exception("Settings are not initialized")

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

    # Only instrument the app when OpenTelemetry is enabled
    FastAPIInstrumentor.instrument_app(
        app=app, tracer_provider=trace.get_tracer_provider(), meter_provider=metrics.get_meter_provider()
    )
    HTTPXClientInstrumentor().instrument(meter_provider=metrics.get_meter_provider())
    LoggingInstrumentor(tracer_provider=trace.get_tracer_provider()).instrument(
        set_logging_format=True
    )
