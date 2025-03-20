
import { Logger, SeverityNumber } from "@opentelemetry/api-logs";
import localLogger from "../instrumentation/localLogger.js";
import { telemetryManager } from "../instrumentation/telemetryManager.js";

export const logger = {
  async getLogger(): Promise<Logger> {
    return await telemetryManager.getLogger();
  },
  emit: async (severityNumber: SeverityNumber, msg: string, ...args: any[]) => {
    const loggerInstance = await logger.getLogger();
    loggerInstance.emit({ severityNumber: severityNumber, body: msg, attributes: { args } });
  },
  info: async (msg: string, ...args: any[]) => {
    localLogger.info(msg, ...args)
    logger.emit(SeverityNumber.INFO, msg, ...args);
  },
  error: async (msg: string, ...args: any[]) => {
    localLogger.error(msg, ...args)
    logger.emit(SeverityNumber.ERROR, msg, ...args);
  },
  warn: async (msg: string, ...args: any[]) => {
    localLogger.warn(msg, ...args)
    logger.emit(SeverityNumber.WARN, msg, ...args);
  },
  debug: async (msg: string, ...args: any[]) => {
    localLogger.debug(msg, ...args)
    logger.emit(SeverityNumber.DEBUG, msg, ...args);
  },
};