/**
 * Instrumentation utilities for performance monitoring and tracing.
 */
import { OTLPLogExporter } from "@opentelemetry/exporter-logs-otlp-http";
import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-http";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { FastifyInstrumentation } from "@opentelemetry/instrumentation-fastify";
import { HttpInstrumentation } from "@opentelemetry/instrumentation-http";
import { Resource } from "@opentelemetry/resources";
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
import { getAuthenticationHeaders } from "../authentication/authentication.js";
import { Instrumentation, registerInstrumentations } from "@opentelemetry/instrumentation";
import { getSettings } from "./settings.js";
import { metrics } from '@opentelemetry/api';
import { LangChainInstrumentation } from "@traceloop/instrumentation-langchain";
import { PinoInstrumentation } from "@opentelemetry/instrumentation-pino";
let tracerProvider: NodeTracerProvider | null = null;
let meterProvider: MeterProvider | null = null;
let loggerProvider: LoggerProvider | null = null;

type InstrumentationInfo = {
    modulePath: string;
    className: string;
    requiredPackages: string[]; // At least one package is required
}

// Initialize instrumentations and log the result
const pinoInstrumentation = new PinoInstrumentation();
const fastifyInstrumentation = new FastifyInstrumentation();
const httpInstrumentation = new HttpInstrumentation();
instrumentApp().then(() => {
}).catch((error) => {
    console.error("Error initializing instrumentation:", error);
});


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

/**
 * Retrieve authentication headers.
 */
async function authHeaders(): Promise<Record<string, string>> {
    const headers = await getAuthenticationHeaders();
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
function getResourceAttributes(): Record<string, any> {
    const settings = getSettings();
    return {
        "service.name": settings.name,
        workspace: settings.workspace,
    }
}
/**
 * Initialize and return the OTLP Metric Exporter.
 */
async function getMetricExporter(): Promise<OTLPMetricExporter | null> {
    const settings = getSettings();
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
    const settings = getSettings();
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
    const settings = getSettings();
    if (!settings.enableOpentelemetry) {
        return null;
    }
    const headers = await authHeaders();
    return new OTLPLogExporter({
        headers: headers,
    });
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

function loadInstrumentation(logger: any): Instrumentation[] {
    let instrumentations: Instrumentation[] = [];
    for (const [name, info] of Object.entries(instrumentationMap)) {
        if (info.requiredPackages.some(pkg => isPackageInstalled(pkg))) {
            importInstrumentationClass(
                info.modulePath,
                info.className
            ).then((module: any) => {
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
                        logger.debug(`Successfully instrumented ${name}`);
                    } catch (error) {
                        logger.debug(`Failed to instrument ${name}: ${error}`);
                    }
                } else {
                    logger.debug(`Could not load instrumentor for ${name}`);
                }
            }).catch((error: any) => {
                logger.debug(`Failed to load instrumentation for ${name}: ${error}`);
            });
        }
    }
    return instrumentations;
}

/**
 * Instrument the Fastify application with OpenTelemetry.
 */
export async function instrumentApp() {
    // Instrument Fastify and HTTP and Pino
    const loggerLib = await import("./logger.js");
    const logger = loggerLib.logger;
    // Dynamically load and enable instrumentations based on installed packages

    const instrumentations = loadInstrumentation(logger);
    instrumentations.push(fastifyInstrumentation);
    instrumentations.push(httpInstrumentation);
    instrumentations.push(pinoInstrumentation);
    const settings = getSettings();
    if (!settings.enableOpentelemetry) {
        return;
    }

    const resource = new Resource(getResourceAttributes());

    // Initialize Tracer Provider with exporter
    //const traceExporter = await getTraceExporter();
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

    // Initialize Logger Provider with exporter
    const logExporter = await getLogExporter();
    if (!logExporter) {
        throw new Error("Log exporter is not initialized");
    }
    loggerProvider = new LoggerProvider({
        resource,
    });
    loggerProvider.addLogRecordProcessor(new BatchLogRecordProcessor(logExporter));

    registerInstrumentations({
        instrumentations: instrumentations,
        loggerProvider: loggerProvider,
        tracerProvider: tracerProvider,
        meterProvider: meterProvider,
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