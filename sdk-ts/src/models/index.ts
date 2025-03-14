import { getLangchainModel } from "./langchain.js";
import { getLlamaIndexModel } from "./llamaindex.js";
import { getVercelAIModel } from "./vercelai.js";

export * from "./langchain.js";
export * from "./llamaindex.js";
export * from "./vercelai.js";

class BLModel {
  modelName: string
  options?: any

  constructor (modelName: string, options?: any) {
    this.modelName = modelName;
    this.options = options||{};
  }

  async ToLangChain() {
    return getLangchainModel(this.modelName, this.options);
  }

  async ToLlamaIndex() {
    return getLlamaIndexModel(this.modelName, this.options);
  }

  async ToVercelAI() {
    return getVercelAIModel(this.modelName, this.options);
  }
}

export const blModel = (modelName: string, options?: any) => {
  return new BLModel(modelName, options);
}