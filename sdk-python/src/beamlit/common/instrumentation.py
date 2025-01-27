import importlib
import logging
from typing import Any, Optional, Type

from fastapi import FastAPI
from opentelemetry import _logs, metrics, trace
from opentelemetry._logs import set_logger_provider
from opentelemetry.exporter.otlp.proto.http._log_exporter import (
    OTLPLogExporter,
)
from opentelemetry.exporter.otlp.proto.http.metric_exporter import (
    OTLPMetricExporter,
)
from opentelemetry.exporter.otlp.proto.http.trace_exporter import (
    OTLPSpanExporter,
)
from opentelemetry.instrumentation.fastapi import (  # type: ignore
    FastAPIInstrumentor,
)
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

from beamlit.authentication import get_authentication_headers

from .settings import get_settings

tracer: trace.Tracer | None = None
meter: metrics.Meter | None = None
logger: LoggerProvider | None = None

log = logging.getLogger(__name__)


def auth_headers() -> Dict[str, str]:
    settings = get_settings()
    headers = get_authentication_headers(settings)
    return {
        "x-beamlit-authorization": headers.get("X-Beamlit-Authorization", ""),
        "x-beamlit-workspace": headers.get("X-Beamlit-Workspace", ""),
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
    resources_dict["workspace"] = settings.workspace
    resources_dict["service.name"] = settings.name
    return resources_dict


def get_metrics_exporter() -> OTLPMetricExporter | None:
    settings = get_settings()
    if not settings.enable_opentelemetry:
        return None
    return OTLPMetricExporter(headers=auth_headers())


def get_span_exporter() -> OTLPSpanExporter | None:
    settings = get_settings()
    if not settings.enable_opentelemetry:
        return None
    return OTLPSpanExporter(headers=auth_headers())


def get_log_exporter() -> OTLPLogExporter | None:
    settings = get_settings()
    if not settings.enable_opentelemetry:
        return None
    return OTLPLogExporter(headers=auth_headers())


def _import_class(module_path: str, class_name: str) -> Optional[Type]:  # type: ignore
    """Dynamically import a class from a module path."""
    try:
        module = importlib.import_module(module_path)
        return getattr(module, class_name)
    except (ImportError, AttributeError) as e:
        log.error(f"Could not import {class_name} from {module_path}: {str(e)}")
        return None


# Define mapping of instrumentor info: (module path, class name, required package)
INSTRUMENTOR_CONFIGS = {
    "httpx": (
        "opentelemetry.instrumentation.httpx",
        "HTTPXClientInstrumentor",
        "httpx",
    ),
    "anthropic": (
        "opentelemetry.instrumentation.anthropic",
        "AnthropicInstrumentor",
        "anthropic",
    ),
    "chroma": (
        "opentelemetry.instrumentation.chroma",
        "ChromaInstrumentor",
        "chromadb",
    ),
    "cohere": (
        "opentelemetry.instrumentation.cohere",
        "CohereInstrumentor",
        "cohere",
    ),
    "groq": ("opentelemetry.instrumentation.groq", "GroqInstrumentor", "groq"),
    "lance": (
        "opentelemetry.instrumentation.lance",
        "LanceInstrumentor",
        "pylance",
    ),
    "langchain": (
        "opentelemetry.instrumentation.langchain",
        "LangchainInstrumentor",
        "langchain",
    ),
    "llama_index": (
        "opentelemetry.instrumentation.llama_index",
        "LlamaIndexInstrumentor",
        "llama_index",
    ),
    "marqo": (
        "opentelemetry.instrumentation.marqo",
        "MarqoInstrumentor",
        "marqo",
    ),
    "milvus": (
        "opentelemetry.instrumentation.milvus",
        "MilvusInstrumentor",
        "pymilvus",
    ),
    "mistralai": (
        "opentelemetry.instrumentation.mistralai",
        "MistralAiInstrumentor",
        "mistralai",
    ),
    "ollama": (
        "opentelemetry.instrumentation.ollama",
        "OllamaInstrumentor",
        "ollama",
    ),
    "openai": (
        "opentelemetry.instrumentation.openai",
        "OpenAIInstrumentor",
        "openai",
    ),
    "pinecone": (
        "opentelemetry.instrumentation.pinecone",
        "PineconeInstrumentor",
        "pinecone",
    ),
    "qdrant": (
        "opentelemetry.instrumentation.qdrant",
        "QdrantInstrumentor",
        "qdrant_client",
    ),
    "replicate": (
        "opentelemetry.instrumentation.replicate",
        "ReplicateInstrumentor",
        "replicate",
    ),
    "together": (
        "opentelemetry.instrumentation.together",
        "TogetherAiInstrumentor",
        "together",
    ),
    "watsonx": (
        "opentelemetry.instrumentation.watsonx",
        "WatsonxInstrumentor",
        "ibm_watson_machine_learning",
    ),
    "weaviate": (
        "opentelemetry.instrumentation.weaviate",
        "WeaviateInstrumentor",
        "weaviate",
    ),
}


def _is_package_installed(package_name: str) -> bool:
    """Check if a package is installed."""
    try:
        importlib.import_module(package_name)
        return True
    except (ImportError, ModuleNotFoundError):
        return False


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
        span_processor = BatchSpanProcessor(get_span_exporter())  # type: ignore
        trace_provider.add_span_processor(span_processor)
        trace.set_tracer_provider(trace_provider)
        tracer = trace_provider.get_tracer(__name__)
    else:
        tracer = trace.get_tracer(__name__)

    # Set up the MeterProvider if not already set
    if not isinstance(metrics.get_meter_provider(), MeterProvider):
        metrics_exporter = PeriodicExportingMetricReader(get_metrics_exporter())  # type: ignore
        meter_provider = MeterProvider(
            resource=resource, metric_readers=[metrics_exporter]
        )
        metrics.set_meter_provider(meter_provider)
        meter = meter_provider.get_meter(__name__)
    else:
        meter = metrics.get_meter(__name__)

    if not isinstance(_logs.get_logger_provider(), LoggerProvider):
        logger_provider = LoggerProvider(resource=resource)
        set_logger_provider(logger_provider)
        logger_provider.add_log_record_processor(
            BatchLogRecordProcessor(get_log_exporter())  # type: ignore
        )
        handler = LoggingHandler(
            level=logging.NOTSET, logger_provider=logger_provider
        )
        logging.getLogger().addHandler(handler)
    else:
        logger_provider = _logs.get_logger_provider()

    # Only instrument the app when OpenTelemetry is enabled
    FastAPIInstrumentor.instrument_app(app)  # type: ignore

    for name, (
        module_path,
        class_name,
        required_package,
    ) in INSTRUMENTOR_CONFIGS.items():
        if _is_package_installed(required_package):
            instrumentor_class = _import_class(module_path, class_name)  # type: ignore
            if instrumentor_class:
                try:
                    instrumentor_class().instrument()
                    log.debug(f"Successfully instrumented {name}")
                except Exception as e:
                    log.debug(f"Failed to instrument {name}: {str(e)}")
            else:
                log.debug(f"Could not load instrumentor for {name}")
        else:
            log.debug(
                f"Skipping {name} instrumentation - required package '{required_package}' not installed"
            )


def shutdown_instrumentation():
    if tracer is not None:
        trace_provider = trace.get_tracer_provider()
        if isinstance(trace_provider, TracerProvider):
            trace_provider.shutdown()
    if meter is not None:
        meter_provider = metrics.get_meter_provider()
        if isinstance(meter_provider, MeterProvider):
            meter_provider.shutdown()
