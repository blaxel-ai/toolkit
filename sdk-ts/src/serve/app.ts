import websocket from "@fastify/websocket";
import { AsyncLocalStorage } from "async_hooks";
import {
  fastify,
  FastifyInstance,
  FastifyReply,
  FastifyRequest,
} from "fastify";
import { IncomingMessage, request } from "http";
import { v4 as uuidv4 } from "uuid";
import { HTTPError } from "../common/error.js";
import { logger } from "../common/logger.js";
import { importModule } from "../common/module.js";
import { getSettings, init } from "../common/settings.js";

/**
 * Custom IncomingMessage extending the standard IncomingMessage.
 */
interface CustomIncomingMessage extends IncomingMessage {
  timeStart: [number, number];
}

/**
 * Creates and configures the Fastify application.
 * @param funcDefault - Optional default function to use.
 * @returns A promise that resolves to a FastifyInstance.
 * @throws Will throw an error if the server module fails to import.
 */
export async function createApp(
  funcDefault: any = null
): Promise<FastifyInstance> {
  const app = fastify();

  const asyncLocalStorage = new AsyncLocalStorage<string>();

  const settings = init();
  logger.info(`Importing server module: ${settings.server.module}`);
  let func = funcDefault || importModule();
  if (!func) {
    throw new Error(
      `Failed to import server module from ${settings.server.module}`
    );
  }
  // Check if function accepts request as first parameter
  const funcParams = func
    .toString()
    .match(/\((.*?)\)/)?.[1]
    .split(",")
    .map((p: string) => p.trim());
  if (!funcParams || funcParams[0] === "") {
    if (func.constructor.name === "AsyncFunction") {
      func = await func();
    } else if (typeof func === "function") {
      func = func();
    }
  }
  logger.info(
    `Running server with environment ${settings.environment} on ${settings.server.host}:${settings.server.port}`
  );

  /**
   * Handles the WebSocket connection.
   * @param socket - The WebSocket connection.
   */
  if (func.stream) {
    logger.info("Starting websocket server");
    app.register(websocket);
    app.register(async function (app: FastifyInstance) {
      app.get("/ws", { websocket: true }, async (socket) => {
        try {
          if (func instanceof Promise) {
            const fn = await func;
            await fn.run(socket, request);
          } else if (typeof func.run === "function") {
            await func.run(socket, request);
          } else if (func.constructor.name === "AsyncFunction") {
            await func(socket, request);
          } else {
            func(socket, request);
          }
        } catch (e) {
          logger.error(e);
        }
      });
    });
  }

  /**
   * Middleware to add a correlation ID to each request.
   */
  app.addHook(
    "onRequest",
    (request: FastifyRequest, reply: FastifyReply, done: () => void) => {
      const requestId = request.headers["x-beamlit-request-id"] || uuidv4();
      (request.raw as CustomIncomingMessage).timeStart = process.hrtime();
      asyncLocalStorage.run(requestId as string, () => {
        reply.header("x-beamlit-request-id", requestId);
        done();
      });
    }
  );

  /**
   * Middleware to add the process time header to each response.
   */
  app.addHook(
    "onResponse",
    (request: FastifyRequest, reply: FastifyReply, done: () => void) => {
      const processTime = process.hrtime(
        (request.raw as CustomIncomingMessage).timeStart
      );
      const processTimeString = `${(
        processTime[0] * 1000 +
        processTime[1] / 1000000
      ).toFixed(2)}ms`;
      reply.header("X-Process-Time", processTimeString);
      const requestId = reply.getHeader("x-beamlit-request-id");
      logger.info(
        `${request.method} ${processTimeString} ${request.url} rid=${requestId}`
      );
      done();
    }
  );

  /**
   * Health check endpoint.
   * @returns An object indicating the status.
   */
  app.get("/health", async () => {
    return { status: "ok" };
  });

  /**
   * Handles POST requests to the root endpoint.
   * @param request - The Fastify request object.
   * @param reply - The Fastify reply object.
   */
  if (!func.stream) {
    app.post("/", async (request: FastifyRequest, reply: FastifyReply) => {
      try {
        let response;
        if (func instanceof Promise) {
          const fn = await func;
          response = await fn.run(request);
        } else if (typeof func.run === "function") {
          response = await func.run(request);
        } else if (func.constructor.name === "AsyncFunction") {
          response = await func(request);
        } else {
          if (typeof func === "function") {
            response = func(request);
          } else {
            response = func;
          }
        }

        if (typeof response === "string") {
          return reply
            .code(200)
            .header("Content-Type", "text/plain")
            .send(response);
        }
        return reply.code(200).send(response);
      } catch (e) {
        if (e instanceof HTTPError) {
          const content = {
            error: e.message,
            status_code: e.status_code,
            ...(settings.environment === "development" && {
              traceback: e.stack,
            }),
          };
          logger.error(`${e.status_code} ${e.stack}`);
          return reply.code(e.status_code).send(content);
        }
        const content = {
          error: `Internal server error, ${e}`,
          ...(settings.environment === "development" && {
            traceback: e instanceof Error ? e.stack : String(e),
          }),
        };
        logger.error(e instanceof Error ? e.stack : String(e));
        return reply.code(500).send(content);
      }
    });
  }

  return app;
}

/**
 * Starts the Fastify application.
 * @param app - The Fastify instance to run.
 * @returns A promise that resolves when the server starts listening.
 */
export async function runApp(app: FastifyInstance) {
  const settings = getSettings();
  return await app.listen({
    host: settings.server.host,
    port: settings.server.port,
  });
}
