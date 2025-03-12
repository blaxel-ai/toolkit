import fs from "fs";
import path from "path";
import { logger } from "../common";
import { AgentBase } from "./base";

/**
 * Recursively retrieves and wraps agents from a specified directory.
 * @param dir - The directory to search for agent files.
 * @param warning - Whether to log warnings on import errors.
 * @returns A promise resolving to an array of AgentBase instances.
 */
export const retrieveWrapperAgent = async (dir: string, warning: boolean) => {
  const agents: AgentBase[] = [];
  const entries = fs.readdirSync(dir, { withFileTypes: true });

  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);

    if (entry.isDirectory()) {
      if (entry.name === "node_modules") continue;
      const agentResources = await retrieveWrapperAgent(fullPath, warning);
      agents.push(...agentResources);
    } else if (entry.name.endsWith(".ts") || entry.name.endsWith(".js")) {
      try {
        const module = await import(`${process.cwd()}/${fullPath}`);
        for (const exportedItem of Object.values(module)) {
          if (
            typeof exportedItem === "function" &&
            exportedItem.toString().includes("wrapAgent")
          ) {
            try {
              const agentBase = (await exportedItem()) as AgentBase;
              agents.push(agentBase);
            } catch {
              // pass
            }
          } else {
            const agentBase = (await exportedItem) as AgentBase;
            agents.push(agentBase);
          }
        }
      } catch (error) {
        if (warning) {
          logger.warn(`Error importing agent from ${fullPath}: ${error}`);
        }
      }
    }
  }
  return agents.filter((agent) => agent.agent);
};
