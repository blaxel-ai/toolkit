from typing import Any

from fastapi import FastAPI
from opentelemetry import metrics, trace
from opentelemetry.exporter.otlp.proto.grpc.metric_exporter import (
    OTLPMetricExporter,
)
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import (
    OTLPSpanExporter,
)
from opentelemetry.instrumentation.fastapi import (  # type: ignore
    FastAPIInstrumentor,
)
from opentelemetry.instrumentation.httpx import (  # type: ignore
    HTTPXClientInstrumentor,
)
from opentelemetry.instrumentation.logging import (  # type: ignore
    LoggingInstrumentor,
)
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import (
    PeriodicExportingMetricReader,
)
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
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


def get_metrics_exporter() -> OTLPMetricExporter:
    return OTLPMetricExporter()


def get_span_exporter() -> OTLPSpanExporter:
    return OTLPSpanExporter()


def instrument_app(app: FastAPI):
    global tracer
    global meter
    settings = get_settings()
    if settings is None:
        raise Exception("Settings are not initialized")
    resource = Resource.create(
        {
            "service.name": settings.name,
            "service.namespace": settings.workspace,
            "service.workspace": settings.workspace,
        }
    )
    # Set up the TracerProvider
    trace_provider = TracerProvider(resource=resource)
    span_processor = BatchSpanProcessor(get_span_exporter())
    trace_provider.add_span_processor(span_processor)
    trace.set_tracer_provider(trace_provider)
    tracer = trace_provider.get_tracer(__name__)

    metrics_exporter = PeriodicExportingMetricReader(get_metrics_exporter())
    meter_provider = MeterProvider(
        resource=resource, metric_readers=[metrics_exporter]
    )
    metrics.set_meter_provider(meter_provider)
    meter = meter_provider.get_meter(__name__)

    FastAPIInstrumentor.instrument_app(  # type: ignore
        app=app, tracer_provider=trace_provider, meter_provider=meter_provider
    )
    HTTPXClientInstrumentor().instrument(meter_provider=meter_provider)  # type: ignore
    LoggingInstrumentor(tracer_provider=trace_provider).instrument(
        set_logging_format=True
    )
