OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp.blaxel.ai \
BL_ENABLE_OPENTELEMETRY=true \
OTEL_TRACES_EXPORTER=otlp \
OTEL_METRICS_EXPORTER=otlp \
OTEL_LOGS_EXPORTER=otlp \
OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp-dev.us-central1.p.gcp.beamlit.net:2083 \
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf \
OTEL_TRACES_SAMPLER=parentbased_traceidratio \
OTEL_TRACES_SAMPLER_ARG=0.1 \
OTEL_RESOURCE_ATTRIBUTES=workload.id=agent-gpt-4o-mini,workspace=main,workload.type=agents,service.group=executionplane,service.domain=execution \
npx tsx integrationtest/customagent.ts --watch