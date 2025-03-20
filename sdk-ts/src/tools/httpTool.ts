import { Function, FunctionSpec } from "../client/index.js"
import { logger } from "../common/logger.js"
import settings from "../common/settings.js"
import { Tool } from "./types.js"
import { schemaToZodSchema } from "./zodSchema.js"

export class HttpTool {
  private spec: FunctionSpec
  private name: string
  constructor(name: string, spec: FunctionSpec) {
    this.name = name
    this.spec = spec
  }

  get url() {
    return new URL(`${settings.runUrl}/${settings.workspace}/functions/${this.name}`)
  }

  async listTools(): Promise<Tool[]> {
    return [{
      name: this.name,
      description: this.spec.description||'',
      inputSchema: schemaToZodSchema(this.spec.schema||{}),
      call: this.call.bind(this)
    }]
  }

  async call(args: any) {
    logger.debug("TOOLCALLING: http", this.name, args)
    const response = await fetch(this.url+"/", {
      method: 'POST',
      headers: {
        ...settings.headers,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(args),
    })
    return await response.text()
  }
}

export const getHttpTool = async (functionData: Function): Promise<Tool[]> => {
  if(!functionData.spec) {
    throw new Error("Function spec is required")
  }
  const httpTool = new HttpTool(functionData.metadata?.name || "", functionData.spec)
  return await httpTool.listTools()
}