/**
 * Instrumentation utilities for performance monitoring and tracing.
 */
import { OTLPLogExporter } from "@opentelemetry/exporter-logs-otlp-http";
import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-http";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { FastifyInstrumentation } from "@opentelemetry/instrumentation-fastify";
import { HttpInstrumentation } from "@opentelemetry/instrumentation-http";
import { envDetector, Resource } from "@opentelemetry/resources";
import {
    BatchLogRecordProcessor,
    LoggerProvider,
} from "@opentelemetry/sdk-logs";
import {
    MeterProvider,
    PeriodicExportingMetricReader,
} from "@opentelemetry/sdk-metrics";
import { AlwaysOnSampler, BatchSpanProcessor } from "@opentelemetry/sdk-trace-base";
import { NodeTracerProvider } from "@opentelemetry/sdk-trace-node";
import { Instrumentation, registerInstrumentations } from "@opentelemetry/instrumentation";
import { metrics } from '@opentelemetry/api';
import { LangChainInstrumentation } from "@traceloop/instrumentation-langchain";
import { PinoInstrumentation } from "@opentelemetry/instrumentation-pino";
import { logs, Logger } from '@opentelemetry/api-logs';


let tracerProvider: NodeTracerProvider | null = null;
let meterProvider: MeterProvider | null = null;
let loggerProvider: LoggerProvider | null = null;

type InstrumentationInfo = {
    modulePath: string;
    className: string;
    requiredPackages: string[]; // At least one package is required
}

let otelLogger: Logger | null = null;

// Define mapping of instrumentor info: (module path, class name, required package)
const instrumentationMap: Record<string, InstrumentationInfo> = {
    "anthropic": {
        modulePath: "@traceloop/instrumentation-anthropic",
        className: "AnthropicInstrumentation",
        requiredPackages: ["anthropic-ai/sdk"]
    },
    "azure": {
        modulePath: "@traceloop/instrumentation-azure",
        className: "AzureInstrumentation",
        requiredPackages: ["azure/openai"]
    },
    "bedrock": {
        modulePath: "@traceloop/instrumentation-bedrock",
        className: "BedrockInstrumentation",
        requiredPackages: ["aws-sdk/client-bedrock-runtime"]
    },
    "chromadb": {
        modulePath: "@traceloop/instrumentation-chromadb",
        className: "ChromaDBInstrumentation",
        requiredPackages: ["chromadb"]
    },
    "cohere": {
        modulePath: "@traceloop/instrumentation-cohere",
        className: "CohereInstrumentation",
        requiredPackages: ["cohere-js"]
    },
    "langchain": {
        modulePath: "@traceloop/instrumentation-langchain",
        className: "LangChainInstrumentation",
        requiredPackages: ["langchain", "@langchain/core", "@langchain/community", "@langchain/langgraph"]
    },
    "llamaindex": {
        modulePath: "@traceloop/instrumentation-llamaindex",
        className: "LlamaIndexInstrumentation",
        requiredPackages: ["llamaindex"]
    },
    "openai": {
        modulePath: "@traceloop/instrumentation-openai",
        className: "OpenAIInstrumentation",
        requiredPackages: ["openai"]
    },
    "pinecone": {
        modulePath: "@traceloop/instrumentation-pinecone",
        className: "PineconeInstrumentation",
        requiredPackages: ["pinecone-database/pinecone"]
    },
    "qdrant": {
        modulePath: "@traceloop/instrumentation-qdrant",
        className: "QdrantInstrumentation",
        requiredPackages: ["qdrant/js-client-rest"]
    },
    "vertexai": {
        modulePath: "@traceloop/instrumentation-vertexai",
        className: "VertexAIInstrumentation",
        requiredPackages: ["google-cloud/aiplatform"]
    }
}

let isInstrumentationInitialized = false;

instrumentApp().then(() => {
}).catch((error) => {
    console.error("Error initializing instrumentation:", error);
});

process.on("SIGINT", () => {
    shutdownInstrumentation().catch(error => {
        console.debug("Fatal error during shutdown:", error);
        process.exit(0);
    });
});

process.on("SIGTERM", () => {
    shutdownInstrumentation().catch(error => {
        console.debug("Fatal error during shutdown:", error);
        process.exit(0);
    });
});




/**
 * Retrieve authentication headers.
 */
async function authHeaders(): Promise<Record<string, string>> {
    const getAuthenticationHeaders = require("../authentication/authentication.js");
    const headers = await getAuthenticationHeaders.getAuthenticationHeaders();
    return {
        "x-beamlit-authorization": headers?.["X-Beamlit-Authorization"] || "",
        "x-beamlit-workspace": headers?.["X-Beamlit-Workspace"] || "",
    };
}
/**
 * Initialize and return the LoggerProvider.
 */
export function getLoggerProviderInstance(): LoggerProvider {
    if (!loggerProvider) {
        throw new Error("LoggerProvider is not initialized");
    }
    return loggerProvider;
}
/**
 * Get resource attributes for OpenTelemetry.
 */
async function getResourceAttributes(): Promise<Record<string, any>> {
    const getSettings = require("./settings.js");
    const settings = getSettings.getSettings();
    const resource = await envDetector.detect()
    return {
        ...resource.attributes,
        "service.name": settings.name,
        workspace: settings.workspace,
    }
}
/**
 * Initialize and return the OTLP Metric Exporter.
 */
async function getMetricExporter(): Promise<OTLPMetricExporter | null> {
    const getSettings = require("./settings.js");
    const settings = getSettings.getSettings();
    if (!settings.enableOpentelemetry) {
        return null;
    }
    const headers = await authHeaders();
    return new OTLPMetricExporter({
        headers: headers,
    });
}
/**
 * Initialize and return the OTLP Trace Exporter.
 */
async function getTraceExporter(): Promise<OTLPTraceExporter | null> {
    const getSettings = require("./settings.js");
    const settings = getSettings.getSettings();
    if (!settings.enableOpentelemetry) {
        return null;
    }
    const headers = await authHeaders();
    return new OTLPTraceExporter({
        headers: headers,
    });
}
/**
 * Initialize and return the OTLP Log Exporter.
 */
async function getLogExporter(): Promise<OTLPLogExporter | null> {
    const getSettings = require("./settings.js");
    const settings = getSettings.getSettings();
    if (!settings.enableOpentelemetry) {
        return null;
    }
    const headers = await authHeaders();
    return new OTLPLogExporter({
        headers: headers,
    });
}

export function getLogger(): Logger {
    if (!otelLogger) {
        throw new Error("Logger is not initialized");
    }
    return otelLogger;
}

/**
 * Check if a package is installed
 */
function isPackageInstalled(packageName: string): boolean {
    try {
        require.resolve(packageName);
        return true;
    } catch (e) {
        return false;
    }
}

/**
 * Import and instantiate an instrumentation class
 */
async function importInstrumentationClass(modulePath: string, className: string): Promise<any> {
    try {
        const module = await import(modulePath);
        return module[className];
    } catch (e) {
        console.debug(`Could not import ${className} from ${modulePath}: ${e}`);
        return null;
    }
}

async function loadInstrumentation(): Promise<Instrumentation[]> {
    let instrumentations: Instrumentation[] = [];
    for (const [name, info] of Object.entries(instrumentationMap)) {
        if (info.requiredPackages.some(pkg => isPackageInstalled(pkg))) {
            const module = await importInstrumentationClass(
                info.modulePath,
                info.className
            );
            if (module) {
                try {
                    const instrumentor = new module() as Instrumentation;
                    instrumentor.enable();
                    instrumentations.push(instrumentor);
                    if (name === "langchain") {
                        const langchain = instrumentor as LangChainInstrumentation;

                        const RunnableModule = require("@langchain/core/runnables");
                        const ToolsModule = require("@langchain/core/tools");
                        const ChainsModule = require("langchain/chains");
                        const AgentsModule = require("langchain/agents");
                        const VectorStoresModule = require("@langchain/core/vectorstores");

                        langchain.manuallyInstrument(
                            {
                                runnablesModule: RunnableModule,
                                toolsModule: ToolsModule,
                                chainsModule: ChainsModule,
                                agentsModule: AgentsModule,
                                vectorStoreModule: VectorStoresModule,
                            }
                        );
                    }
                } catch (error) {
                    console.debug(`Failed to instrument ${name}: ${error}`);
                }
            }
        }
    }
    return instrumentations;
}


/**
 * Instrument the Fastify application with OpenTelemetry.
 */
export async function instrumentApp() {
    if (!process.env.BL_ENABLE_OPENTELEMETRY || isInstrumentationInitialized) {
        return;
    }
    isInstrumentationInitialized = true;

    const pinoInstrumentation = new PinoInstrumentation();
    const fastifyInstrumentation = new FastifyInstrumentation();
    const httpInstrumentation = new HttpInstrumentation();
    
    const instrumentations = await loadInstrumentation();

    instrumentations.push(fastifyInstrumentation);
    instrumentations.push(httpInstrumentation);
    instrumentations.push(pinoInstrumentation);

    const resource = new Resource(await getResourceAttributes());


    // Initialize Logger Provider with exporter
    const logExporter = await getLogExporter();
    if (!logExporter) {
        throw new Error("Log exporter is not initialized");
    }
    loggerProvider = new LoggerProvider({
        resource,
    });
    loggerProvider.addLogRecordProcessor(new BatchLogRecordProcessor(logExporter));
    logs.setGlobalLoggerProvider(loggerProvider);




    // Initialize Tracer Provider with exporter
    const traceExporter = await getTraceExporter();
    if (!traceExporter) {
        throw new Error("Trace exporter is not initialized");
    }
    tracerProvider = new NodeTracerProvider({
        resource,
        sampler: new AlwaysOnSampler(),
        spanProcessors: [new BatchSpanProcessor(traceExporter)],
    });
    tracerProvider.register(); // This registers it as the global tracer provider

    // Initialize Meter Provider with exporter
    const metricExporter = await getMetricExporter();
    if (!metricExporter) {
        throw new Error("Metric exporter is not initialized");
    }
    meterProvider = new MeterProvider({
        resource,
        readers: [
            new PeriodicExportingMetricReader({
                exporter: metricExporter,
                exportIntervalMillis: 60000,
            })
        ]
    });
    // Register as global meter provider
    metrics.setGlobalMeterProvider(meterProvider);

    registerInstrumentations({
        instrumentations: instrumentations,
    });
}

/**
 * Shutdown OpenTelemetry instrumentation.
 */
async function shutdownInstrumentation() {
    try {
        const shutdownPromises = [];
        if (tracerProvider) {
            shutdownPromises.push(
                tracerProvider.shutdown()
                    .catch(error => console.debug("Error shutting down tracer provider:", error))
            );
        }

        if (meterProvider) {
            shutdownPromises.push(
                meterProvider.shutdown()
                    .catch(error => console.debug("Error shutting down meter provider:", error))
            );
        }

        if (loggerProvider) {
            shutdownPromises.push(
                loggerProvider.shutdown()
                    .catch(error => console.debug("Error shutting down logger provider:", error))
            );
        }

        // Wait for all providers to shutdown with a timeout
        await Promise.race([
            Promise.all(shutdownPromises),
            new Promise(resolve => setTimeout(resolve, 5000)) // 5 second timeout
        ]);

        process.exit(0);
    } catch (error) {
        console.error("Error during shutdown:", error);
        process.exit(1);
    }
}