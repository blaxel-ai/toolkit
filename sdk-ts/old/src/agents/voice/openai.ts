import { StructuredTool } from "@langchain/core/tools";
import WebSocket from "ws";
import { zodToJsonSchema } from "zod-to-json-schema";
import { logger } from "../../common/logger";
import { createStreamFromWebsocket, mergeStreams } from "./utils/stream";

const EVENTS_TO_IGNORE = [
  "response.function_call_arguments.delta",
  "rate_limits.updated",
  "response.audio_transcript.delta",
  "response.created",
  "response.content_part.added",
  "response.content_part.done",
  "conversation.item.created",
  "response.audio.done",
  "session.created",
  "session.updated",
  "response.done",
  "response.output_item.done",
];

/**
 * Manages the WebSocket connection to the OpenAI API for voice interactions.
 */
class OpenAIWebSocketConnection {
  ws?: WebSocket;
  url: string;
  headers: Record<string, string>;
  model: string;

  /**
   * Constructs a new OpenAIWebSocketConnection instance.
   * @param params - Configuration parameters including URL, headers, and model name.
   */
  constructor(params: {
    url: string;
    headers: Record<string, string>;
    model: string;
  }) {
    this.url = params.url;
    this.headers = params.headers;
    this.model = params.model;
  }

  /**
   * Establishes a WebSocket connection with the OpenAI API.
   * @throws If the connection fails or times out.
   */
  async connect() {
    const finalUrl = `${this.url}/realtime?model=${this.model}`;
    this.ws = new WebSocket(finalUrl, {
      headers: { ...this.headers, "OpenAI-Beta": "realtime=v1" },
    });
    await new Promise<void>((resolve, reject) => {
      const timeout = setTimeout(() => {
        reject(new Error("Connection timed out after 10 seconds."));
      }, 10000);

      this.ws?.once("open", () => {
        clearTimeout(timeout);
        resolve();
      });

      this.ws?.once("error", (error: unknown) => {
        clearTimeout(timeout);
        reject(error);
      });
    });
  }

  /**
   * Sends an event to the OpenAI API via WebSocket.
   * @param event - The event data to send.
   * @throws If the WebSocket connection is not active.
   */
  sendEvent(event: Record<string, unknown>) {
    const formattedEvent = JSON.stringify(event);
    if (this.ws === undefined) {
      throw new Error("Socket connection is not active, call .connect() first");
    }
    this.ws?.send(formattedEvent);
  }

  /**
   * Creates an async generator to stream events from the WebSocket.
   * @returns An async generator yielding chat completion chunks.
   * @throws If the WebSocket connection is not active.
   */
  async *eventStream() {
    if (!this.ws) {
      throw new Error("Socket connection is not active, call .connect() first");
    }
    yield* createStreamFromWebsocket(this.ws);
  }
}

/**
 * Executes tools based on incoming tool calls and handles their outputs.
 */
class VoiceToolExecutor {
  protected toolsByName: Record<string, StructuredTool>;
  protected triggerPromise: Promise<any> | null = null;
  protected triggerResolve: ((value: any) => void) | null = null;
  protected lock: Promise<void> | null = null;

  /**
   * Constructs a new VoiceToolExecutor instance.
   * @param toolsByName - A mapping of tool names to StructuredTool instances.
   */
  constructor(toolsByName: Record<string, StructuredTool>) {
    this.toolsByName = toolsByName;
  }

  /**
   * Triggers the execution of a tool function.
   * @returns A promise that resolves when the tool is triggered.
   */
  protected async triggerFunc(): Promise<any> {
    if (!this.triggerPromise) {
      this.triggerPromise = new Promise((resolve) => {
        this.triggerResolve = resolve;
      });
    }
    return this.triggerPromise;
  }

  /**
   * Adds a tool call to be processed.
   * @param toolCall - The tool call data.
   * @throws If a tool call is already in progress.
   */
  async addToolCall(toolCall: any): Promise<void> {
    while (this.lock) {
      await this.lock;
    }

    this.lock = (async () => {
      if (this.triggerResolve) {
        this.triggerResolve(toolCall);
        this.triggerPromise = null;
        this.triggerResolve = null;
      } else {
        throw new Error("Tool call adding already in progress");
      }
    })();

    await this.lock;
    this.lock = null;
  }

  /**
   * Creates a task to execute a tool call.
   * @param toolCall - The tool call data.
   * @returns The result of the tool call.
   * @throws If the tool is not found or arguments are invalid.
   */
  protected async createToolCallTask(toolCall: any): Promise<any> {
    const tool = this.toolsByName[toolCall.name];
    if (!tool) {
      throw new Error(
        `Tool ${toolCall.name} not found. Must be one of ${Object.keys(
          this.toolsByName
        )}`
      );
    }

    let args;
    try {
      args = JSON.parse(toolCall.arguments);
    } catch {
      throw new Error(
        `Failed to parse arguments '${toolCall.arguments}'. Must be valid JSON.`
      );
    }

    const result = await tool.call(args);
    const resultStr =
      typeof result === "string" ? result : JSON.stringify(result);

    return {
      type: "conversation.item.create",
      item: {
        id: toolCall.call_id,
        call_id: toolCall.call_id,
        type: "function_call_output",
        output: resultStr,
      },
    };
  }

  /**
   * An async generator that yields tool execution results.
   * @returns An async generator yielding chat completion chunks.
   */
  async *outputIterator(): AsyncGenerator<any, void, unknown> {
    while (true) {
      const toolCall = await this.triggerFunc();
      try {
        const result = await this.createToolCallTask(toolCall);
        yield result;
      } catch (error: any) {
        yield {
          type: "conversation.item.create",
          item: {
            id: toolCall.call_id,
            call_id: toolCall.call_id,
            type: "function_call_output",
            output: `Error: ${error.message}`,
          },
        };
      }
    }
  }
}

/**
 * Represents an OpenAI Voice React Agent for handling voice interactions.
 */
export class OpenAIVoiceReactAgent {
  protected connection: OpenAIWebSocketConnection;

  protected instructions?: string;

  protected tools: StructuredTool[];

  /**
   * Constructs a new OpenAIVoiceReactAgent instance.
   * @param params - Configuration parameters including URL, model, headers, instructions, and tools.
   */
  constructor(params: {
    url: string;
    model: string;
    headers: Record<string, string>;
    instructions?: string;
    tools?: StructuredTool[];
  }) {
    this.connection = new OpenAIWebSocketConnection({
      url: params.url.replace("http", "ws"),
      headers: params.headers,
      model: params.model,
    });
    this.instructions = params.instructions;
    this.tools = params.tools ?? [];
  }

  /**
   * Binds a set of tools to the agent.
   * @param tools - An array of StructuredTool instances.
   */
  bindTools(tools: StructuredTool[]) {
    this.tools = tools;
  }

  /**
   * Connects to the OpenAI API and handles sending and receiving messages.
   * @param websocketOrStream - An async generator or WebSocket instance for input.
   * @param sendOutputChunk - A callback function to send output chunks.
   */
  async connect(
    websocketOrStream: AsyncGenerator<string> | WebSocket,
    sendOutputChunk: (chunk: string) => void | Promise<void>
  ) {
    let inputStream;
    if ("next" in websocketOrStream) {
      inputStream = websocketOrStream;
    } else {
      inputStream = createStreamFromWebsocket(websocketOrStream);
    }
    const toolsByName = this.tools.reduce(
      (toolsByName: Record<string, StructuredTool>, tool) => {
        toolsByName[tool.name] = tool;
        return toolsByName;
      },
      {}
    );
    const toolExecutor = new VoiceToolExecutor(toolsByName);
    await this.connection.connect();
    const modelReceiveStream = this.connection.eventStream();
    // Send tools and instructions with initial chunk
    const toolDefs = Object.values(toolsByName).map((tool) => ({
      type: "function",
      name: tool.name,
      description: tool.description,
      parameters: zodToJsonSchema(tool.schema),
    }));

    this.connection.sendEvent({
      type: "session.update",
      session: {
        instructions: this.instructions,
        input_audio_transcription: {
          model: "whisper-1",
        },
        tools: toolDefs,
      },
    });
    for await (const [streamKey, dataRaw] of mergeStreams({
      input_mic: inputStream,
      output_speaker: modelReceiveStream,
      tool_outputs: toolExecutor.outputIterator(),
    })) {
      let data: any;
      try {
        data = typeof dataRaw === "string" ? JSON.parse(dataRaw) : dataRaw;
      } catch {
        logger.error("Error decoding data:", dataRaw);
        continue;
      }

      if (streamKey === "input_mic") {
        this.connection.sendEvent(data);
      } else if (streamKey === "tool_outputs") {
        logger.info("tool output", data);
        this.connection.sendEvent(data);
        this.connection.sendEvent({ type: "response.create", response: {} });
      } else if (streamKey === "output_speaker") {
        const { type } = data;
        if (type === "response.audio.delta") {
          sendOutputChunk(JSON.stringify(data));
        } else if (type === "response.audio_buffer.speech_started") {
          logger.info("interrupt");
          sendOutputChunk(JSON.stringify(data));
        } else if (type === "error") {
          console.error("error:", data);
        } else if (type === "response.function_call_arguments.done") {
          logger.info("tool call", data);
          toolExecutor.addToolCall(data);
        } else if (type === "response.audio_transcript.done") {
          logger.info("model:", data.transcript);
        } else if (
          type === "conversation.item.input_audio_transcription.completed"
        ) {
          logger.info("user:", data.transcript);
        } else if (!EVENTS_TO_IGNORE.includes(type)) {
          logger.info(type);
        }
      }
    }
  }
}
