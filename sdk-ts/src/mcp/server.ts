// Dans votre SDK
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { Transport } from "@modelcontextprotocol/sdk/shared/transport.js";
import { logger } from "../common/logger";
import { BlaxelMcpServerTransport } from "./websocket";

const originalConnect = McpServer.prototype.connect;
McpServer.prototype.connect = async function (transport: Transport): Promise<void> {
  if (process.env.BL_SERVER_PORT) {
    logger.info("Starting WebSocket Server on port " + process.env.BL_SERVER_PORT);
    const port = parseInt(process.env.BL_SERVER_PORT ?? '8080', 10);
    const blaxelTransport = new BlaxelMcpServerTransport(port);
    return originalConnect.call(this, blaxelTransport);
  }
  return originalConnect.call(this, transport);
};

export { BlaxelMcpServerTransport, McpServer };

