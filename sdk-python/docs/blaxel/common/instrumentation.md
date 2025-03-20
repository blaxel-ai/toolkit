Module blaxel.common.instrumentation
====================================
This module provides utilities for setting up and managing OpenTelemetry instrumentation within Blaxel.
It includes classes and functions for configuring tracers, meters, loggers, and integrating with FastAPI applications.

Functions
---------

`auth_headers() ‑> Dict[str, str]`
:   Retrieves authentication headers based on the current settings.
    
    Returns:
        Dict[str, str]: A dictionary containing authentication headers.

`get_log_exporter() ‑> opentelemetry.exporter.otlp.proto.http._log_exporter.OTLPLogExporter | None`
:   

`get_logger() ‑> opentelemetry.sdk._logs._internal.LoggerProvider`
:   Retrieves the current logger provider.
    
    Returns:
        LoggerProvider: The active logger provider.
    
    Raises:
        Exception: If the logger has not been initialized.

`get_metrics_exporter() ‑> opentelemetry.exporter.otlp.proto.http.metric_exporter.OTLPMetricExporter | None`
:   

`get_resource_attributes() ‑> Dict[str, Any]`
:   

`get_span_exporter() ‑> opentelemetry.exporter.otlp.proto.http.trace_exporter.OTLPSpanExporter | None`
:   

`instrument_app(app: fastapi.applications.FastAPI)`
:   Instruments the given FastAPI application with OpenTelemetry.
    
    This includes setting up tracer and meter providers, configuring exporters, and instrumenting
    various modules based on available packages.
    
    Parameters:
        app (FastAPI): The FastAPI application to instrument.

`shutdown_instrumentation()`
:   Shuts down the OpenTelemetry instrumentation providers gracefully.
    
    This ensures that all spans and metrics are properly exported before the application exits.