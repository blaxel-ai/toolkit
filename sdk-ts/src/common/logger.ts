import { pino } from "pino";

/**
 * Configuration for the logger.
 */
export const loggerConfiguration = {
  level: process.env.BL_LOG_LEVEL || "info",
  transport: {
    targets: [
      {
        target: "pino-opentelemetry-transport",
        options: {
          colorizeObjects: false,
          translateTime: false,
          hideObject: true,
          messageFormat: "\x1B[37m{msg}",
          ignore: "pid,hostname,time",
        },
      },
      {
        target: "pino-pretty",
        options: {
          colorizeObjects: false,
          translateTime: false,
          hideObject: true,
          messageFormat: "\x1B[37m{msg}",
          ignore: "pid,hostname,time",
        },
      },
    ],
  },
};

/**
 * Logger instance for the application.
 */
export const logger = pino(loggerConfiguration);
