import { getFunction } from "../client/index.js";
import { getHttpTool } from "./httpTool.js";
import { getMcpTool } from "./mcpTool.js";
import { Tool } from "./types.js";

export * from "./langchain.js";
export * from "./llamaindex.js";

export const getTool = async (name: string): Promise<Tool[]> => {
  const {data:functionData} = await getFunction({path: {functionName: name}})
  if(!functionData) {
    throw new Error(`Function ${name} not found`)
  }
  if (functionData?.spec?.runtime?.type === "mcp") {
    return await getMcpTool(functionData)
  }
  return await getHttpTool(functionData)
}