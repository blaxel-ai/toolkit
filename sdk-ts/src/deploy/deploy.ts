import { Client } from "@hey-api/client-fetch";
import fs from "fs";
import path from "path";
import yaml from "yaml";
import { AgentBase } from "../agents/base";
import { retrieveWrapperAgent } from "../agents/common";
import { newClient } from "../authentication";
import { Agent, Function, getAgent } from "../client";
import { logger } from "../common";
import { init, Settings } from "../common/settings";
import { slugify } from "../common/slugify";
import { FunctionBase } from "../functions/base";
import { retrieveWrapperFunction } from "../functions/common";

/**
 * Generates a Dockerfile for the specified resource.
 * @param settings - The current application settings.
 * @param resourceType - Type of resource ('agent' or 'function').
 * @param resource - The resource configuration.
 * @returns A string containing the Dockerfile content.
 */
const generateDockerfile = (
  settings: Settings,
  resourceType: "agent" | "function",
  resource: Agent | Function
) => {
  const cmd = ["bl", "serve", "--port", "80", "--module"];
  if (resourceType === "agent") {
    cmd.push(
      `${settings.server.directory}/${settings.server.module}`.replaceAll(
        "/",
        "."
      )
    );
    cmd.push("--remote");
  }
  if (resourceType === "function") {
    cmd.push(
      `${settings.agent.functionsDirectory}/${resource.metadata?.name}.doNotRemove`.replaceAll(
        "/",
        "."
      )
    );
  }
  const cmdStr = cmd.map((c) => `"${c}"`).join(",");

  const CLI_INSTALL_URL = "https://raw.githubusercontent.com/beamlit/toolkit/main/install.sh";
  // if (process.env.BL_ENV === "dev") {
  //   CLI_INSTALL_URL = "https://raw.githubusercontent.com/cploujoux/toolkit/main/install.sh";
  // }

  return `
FROM node:20-slim

RUN apt update && apt install -y curl build-essential

RUN curl -fsSL ${CLI_INSTALL_URL} | BINDIR=/bin sh
WORKDIR /blaxel

# Install dependencies
COPY package.json /blaxel/package.json
COPY package-lock.json /blaxel/package-lock.json
RUN npm i

# Copy source code and utils files
COPY README.m[d] /blaxel/README.md
COPY LICENS[E] /blaxel/LICENSE
COPY tsconfig.jso[n] /blaxel/tsconfig.json
COPY ${settings.server.directory} /blaxel/src
COPY index.t[s] /blaxel/index.ts

RUN npm run build
RUN cp -r dist/* /blaxel

ENV COMMAND="node index.js"

ENTRYPOINT [${cmdStr}]
`;
};

/**
 * Generates Dockerfiles and configurations for functions within the specified directory.
 * @param settings - The current application settings.
 * @param directory - The directory to output the generated Dockerfiles.
 * @returns An array of FunctionBase instances.
 */
const generateFunctions = async (settings: Settings, directory: string) => {
  const functions = await retrieveWrapperFunction(
    settings.agent.functionsDirectory,
    false
  );
  functions.forEach((func) => {
    const functionConfiguration = func.function;
    const dockerfile = generateDockerfile(
      settings,
      "function",
      functionConfiguration
    );
    if (functionConfiguration.metadata?.name) {
      const funcName = slugify(functionConfiguration.metadata.name);
      functionConfiguration.metadata.name = funcName;
      functionConfiguration.metadata.labels =
        functionConfiguration.metadata.labels || {};
      functionConfiguration.metadata.labels["x-blaxel-auto-generated"] =
        "true";
      if (!fs.existsSync(`${directory}/functions`)) {
        fs.mkdirSync(`${directory}/functions`);
      }
      if (!fs.existsSync(`${directory}/functions/${funcName}`)) {
        fs.mkdirSync(`${directory}/functions/${funcName}`);
      }
      fs.writeFileSync(
        `${directory}/functions/${funcName}/Dockerfile`,
        dockerfile
      );
      fs.writeFileSync(
        `${directory}/functions/${funcName}/function.yaml`,
        yaml.stringify({
          apiVersion: "blaxel.ai/v1alpha1",
          kind: "Function",
          ...functionConfiguration,
        })
      );
    }
  });
  return functions;
};

const generateAgents = async (
  settings: Settings,
  directory: string,
  functionsNames: string[],
  client: Client,
  forceName?: string
) => {
  const agentDirectory = settings.server.directory;
  if (!fs.existsSync(agentDirectory))
    throw new Error(`Agent directory ${agentDirectory} not found`);
  const agents = await retrieveWrapperAgent(agentDirectory, false);

  await Promise.all(
    agents.map(async (agent) => {
      const agentConfiguration = agent.agent;
      if (agentConfiguration && agentConfiguration.metadata?.name) {
        if (forceName) {
          agentConfiguration.metadata.name = forceName;
        }
        const dockerfile = generateDockerfile(
          settings,
          "agent",
          agentConfiguration
        );
        try {
          const { data } = await getAgent({
            client,
            path: { agentName: agentConfiguration.metadata.name },
          });
          agentConfiguration.spec!.repository = data?.spec?.repository;
        } catch (error) {
          logger.error(
            `Error retrieving agent ${agentConfiguration.metadata.name}: ${error}`
          );
        }
        const remoteFunctions = agent.remoteFunctions || [];
        const existingFunctions = agentConfiguration.spec?.functions || [];
        const agentName = slugify(agentConfiguration.metadata.name);
        agentConfiguration.metadata.name = agentName;
        agentConfiguration.spec!.functions = [
          ...new Set([
            ...functionsNames,
            ...remoteFunctions,
            ...existingFunctions,
          ]),
        ];
        agentConfiguration.metadata.labels =
          agentConfiguration.metadata.labels || {};
        agentConfiguration.metadata.labels["x-blaxel-auto-generated"] = "true";

        if (!fs.existsSync(`${directory}/agents`)) {
          fs.mkdirSync(`${directory}/agents`);
        }
        if (!fs.existsSync(`${directory}/agents/${agentName}`)) {
          fs.mkdirSync(`${directory}/agents/${agentName}`);
        }
        fs.writeFileSync(
          `${directory}/agents/${agentName}/Dockerfile`,
          dockerfile
        );
        fs.writeFileSync(
          `${directory}/agents/${agentName}/agent.yaml`,
          yaml.stringify({
            apiVersion: "blaxel.ai/v1alpha1",
            kind: "Agent",
            ...agentConfiguration,
          })
        );
      }
    })
  );
  return agents;
};

/**
 * Cleans up auto-generated deployment files that are no longer needed.
 * @param directory - The directory where deployments are located.
 * @param type - The type of deployment ('agent' or 'function').
 * @param deployments - The list of current deployments.
 */
const cleanAutoGenerated = (
  directory: string,
  type: "agent" | "function",
  deployments: AgentBase[] | FunctionBase[]
) => {
  const deployDir = path.join(directory, `${type}s`);
  const deployNames = deployments.map((d) => {
    if ("agent" in d) {
      return d.agent?.metadata?.name;
    } else {
      return d.function?.metadata?.name;
    }
  });

  if (fs.existsSync(deployDir)) {
    fs.readdirSync(deployDir).forEach((itemDir) => {
      const fullPath = path.join(deployDir, itemDir);
      if (
        fs.statSync(fullPath).isDirectory() &&
        !deployNames.includes(itemDir)
      ) {
        const yamlFile = path.join(fullPath, `${type}.yaml`);
        if (fs.existsSync(yamlFile)) {
          try {
            const content = yaml.parse(fs.readFileSync(yamlFile, "utf8"));
            if (
              content?.metadata?.labels?.["x-blaxel-auto-generated"] === "true"
            ) {
              fs.rmSync(fullPath, { recursive: true, force: true });
            }
          } catch {
            return;
          }
        }
      }
    });
  }
};

/**
 * Generates the entire Blaxel deployment configuration, including agents and functions.
 * @param directory - The directory to output the generated deployment files.
 * @param forceName - Optional name to use for the deployment.
 */
export const generateBlaxelDeployment = async (directory: string, forceName?: string) => {
  if (!fs.existsSync(directory)) {
    fs.mkdirSync(directory);
  }
  const settings = init();
  const client = newClient();

  let functions: FunctionBase[] = [];
  if (!fs.existsSync(settings.agent.functionsDirectory)) {
    logger.warn(
      `Functions directory ${settings.agent.functionsDirectory} not found`
    );
    functions = [];
  } else {
    functions = await generateFunctions(settings, directory);
  }

  const functionsNames = functions.map((f) => f.function.metadata?.name || "");
  const agents = await generateAgents(
    settings,
    directory,
    functionsNames,
    client,
    forceName
  );
  cleanAutoGenerated(directory, "function", functions);
  cleanAutoGenerated(directory, "agent", agents);
};
