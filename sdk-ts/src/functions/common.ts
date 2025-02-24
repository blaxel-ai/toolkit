import { Client } from "@hey-api/client-fetch";
import { StructuredTool } from "@langchain/core/tools";
import fs from "fs";
import path from "path";
import z from "zod";
import { ChainToolkit } from "../agents/chain.js";
import { newClient } from "../authentication/authentication.js";
import { AgentChain, StoreFunctionParameter } from "../client/types.gen.js";
import { logger } from "../common/logger.js";
import { getSettings } from "../common/settings.js";
import { RunClient } from "../run.js";
import { FunctionBase } from "./base.js";
import { LocalFunction, LocalToolkit } from "./local.js";
import { RemoteToolkit } from "./remote.js";

/**
 * Converts an array of `StoreFunctionParameter` objects into a Zod schema for validation.
 *
 * @param {StoreFunctionParameter[]} parameters - The parameters to convert.
 * @returns {z.ZodObject<any>} A Zod object schema representing the parameters.
 */
export const parametersToZodSchema = (
  parameters: StoreFunctionParameter[]
): z.ZodObject<any> => {
  const shape: { [key: string]: z.ZodType } = {};

  parameters
    .filter((param) => param.name)
    .forEach((param) => {
      let zodType: z.ZodType;

      switch (param.type) {
        case "boolean":
          zodType = z.boolean();
          break;
        case "number":
          zodType = z.number();
          break;
        default:
          zodType = z.string();
      }

      if (param.description) {
        zodType = zodType.describe(param.description);
      }
      shape[param?.name || ""] = param.required ? zodType : zodType.optional();
    });
  return z.object(shape);
};

/**
 * Options for retrieving functions.
 *
 * @typedef {Object} GetFunctionsOptions
 * @property {string[] | null} [remoteFunctions] - List of remote function names to include.
 * @property {AgentChain[] | null} [chain] - Agent chains to include.
 * @property {Client | null} [client] - Optional client instance.
 * @property {string | null} [dir] - Directory to search for functions.
 * @property {boolean} [warning] - Whether to log warnings on errors.
 */
export type GetFunctionsOptions = {
  remoteFunctions?: string[] | null;
  localFunctions?: LocalFunction[] | null;
  chain?: AgentChain[] | null;
  client?: Client | null;
  dir?: string | null;
  warning?: boolean;
};

const MAX_RETRIES = 10;
const RETRY_DELAY = 1000; // 1 second delay between retries

const initializeWithRetry = async (
  toolkit: RemoteToolkit | LocalToolkit,
  name: string,
  maxRetries: number,
  url?: string
) => {
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    switch (toolkit.constructor.name) {
      case "RemoteToolkit": {
        try {
          await toolkit.initialize(name);
          return await toolkit.getTools();
        } catch (error) {
          if (attempt === maxRetries) {
            logger.warn(
              `Failed to initialize remote function ${name} after ${maxRetries} attempts: ${error}`
            );
            throw error;
          }
          logger.info(`Attempt ${attempt} failed for ${name}, retrying...`);
          await new Promise((resolve) => setTimeout(resolve, RETRY_DELAY));
        }
        break;
      }
      case "LocalToolkit": {
        try {
          if (!url) {
            throw new Error("URL is required for local functions");
          }
          await toolkit.initialize(url);
          return await toolkit.getTools();
        } catch (error) {
          if (attempt === maxRetries) {
            logger.warn(
              `Failed to initialize local function ${name} after ${maxRetries} attempts: ${error}`
            );
            throw error;
          }
          logger.info(`Attempt ${attempt} failed for ${name}, retrying...`);
          await new Promise((resolve) => setTimeout(resolve, RETRY_DELAY));
        }
        break;
      }
    }
  }
  return [];
};

/**
 * Recursively retrieves and wraps functions from the specified directory.
 *
 * @param {string} dir - The directory to scan for function files.
 * @param {boolean} warning - Whether to log warnings on import errors.
 * @returns {Promise<FunctionBase[]>} An array of wrapped `FunctionBase` objects.
 */
export const retrieveWrapperFunction = async (
  dir: string,
  warning: boolean
) => {
  const functions: FunctionBase[] = [];
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  const modules: string[] = [];
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      functions.push(...(await retrieveWrapperFunction(fullPath, warning)));
    } else if (entry.name.endsWith(".ts") || entry.name.endsWith(".js")) {
      try {
        const modulePath = `${path.resolve(fullPath)}`;
        const moduleName = modulePath.replace(".ts", "").replace(".js", "");
        if (modules.includes(moduleName)) {
          continue;
        }
        modules.push(moduleName);
        const module = require(moduleName); // eslint-disable-line
        for (const exportedItem of Object.values(module)) {
          const functionBase = (await exportedItem) as FunctionBase;
          if (functionBase?.tools) {
            functions.push(functionBase);
          }
        }
      } catch (error) {
        if (warning) {
          logger.warn(`Error importing function from ${fullPath}: ${error}`);
        }
      }
    }
  }
  return functions;
};

/**
 * Aggregates available functions based on provided options, including remote functions and agent chains.
 *
 * @param {GetFunctionsOptions} [options={}] - Configuration options for retrieving functions.
 * @returns {Promise<StructuredTool[]>} An array of structured tools representing available functions.
 */
export const getFunctions = async (options: GetFunctionsOptions = {}) => {
  const settings = getSettings();
  let { client, dir } = options;
  const { warning } = options;
  const { remoteFunctions, chain, localFunctions } = options;
  if (!client) {
    client = newClient();
  }
  if (!dir) {
    dir = settings.agent.functionsDirectory;
  }
  const functions: StructuredTool[] = [];

  if (dir && fs.existsSync(dir)) {
    logger.info(`Importing functions from ${dir}`);
    const functionsBlaxel = await retrieveWrapperFunction(
      dir,
      warning ?? false
    );
    functionsBlaxel.forEach((func) => {
      functions.push(...func.tools);
    });
  }

  if (remoteFunctions && !settings.deploy) {
    await Promise.all(
      remoteFunctions.map(async (name) => {
        try {
          const toolkit = new RemoteToolkit(client, name);
          const tools = await initializeWithRetry(toolkit, name, MAX_RETRIES);
          functions.push(...(tools || []));
        } catch (error) {
          logger.warn(`Failed to initialize remote function ${name}: ${error}`);
        }
      })
    );
  }
  if (chain && !settings.deploy) {
    const runClient = new RunClient(client);
    const toolkit = new ChainToolkit(runClient, chain);
    await toolkit.initialize();
    functions.push(...toolkit.getTools());
  }
  if (localFunctions && !settings.deploy) {
    await Promise.all(
      localFunctions.map(async (func) => {
        try {
          const toolkit = new LocalToolkit(client, func.name, func.url);
          const tools = await initializeWithRetry(
            toolkit,
            func.name,
            MAX_RETRIES,
            func.url
          );
          functions.push(...(tools || []));
        } catch (error) {
          logger.warn(
            `Failed to initialize local function ${func.name}: ${error}`
          );
        }
      })
    );
  }
  return functions;
};
