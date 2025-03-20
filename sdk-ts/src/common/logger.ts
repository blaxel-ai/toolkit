/* eslint-disable no-console */
import { Logger, SeverityNumber } from "@opentelemetry/api-logs";
import localLogger from "../instrumentation/localLogger.js";
import { telemetryManager } from "../instrumentation/telemetryManager.js";

export const logger = {
  async getLogger(): Promise<Logger> {
    return await telemetryManager.getLogger();
  },
  emit: async (severityNumber: SeverityNumber, msg: string, ...args: any[]) => {
    // originalConsole.info(msg, ...args);
    const loggerInstance = await logger.getLogger();
    loggerInstance.emit({ severityNumber: severityNumber, body: msg, attributes: { args } });
  },
  info: async (msg: string, ...args: any[]) => {
    // originalConsole.info(msg, ...args);
    logger.emit(SeverityNumber.INFO, msg, ...args);
  },
  error: async (msg: string, ...args: any[]) => {
    // originalConsole.error(msg, ...args);
    logger.emit(SeverityNumber.ERROR, msg, ...args);
  },
  warn: async (msg: string, ...args: any[]) => {
    // originalConsole.warn(msg, ...args);
    logger.emit(SeverityNumber.WARN, msg, ...args);
  },
  debug: async (msg: string, ...args: any[]) => {
    // originalConsole.debug(msg, ...args);
    logger.emit(SeverityNumber.DEBUG, msg, ...args);
  },
};


console.info = (...args) => {
  localLogger.info(...args)
  logger.info(args[0], ...args.slice(1));
}
console.log = (...args) => {
  localLogger.info(...args)
  logger.info(args[0], ...args.slice(1));
}
console.error = (...args) => {
  localLogger.error(...args)
  logger.error(args[0], ...args.slice(1));
}
console.warn = (...args) => {
  localLogger.warn(...args)
  logger.warn(args[0], ...args.slice(1));
}
console.debug = (...args) => {
  localLogger.debug(...args)
  logger.debug(args[0], ...args.slice(1));
}
