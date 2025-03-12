import { Client } from "@hey-api/client-fetch";
import { HTTPError } from "../common/error.js";
import { RunClient } from "../run.js";

export interface EmbeddingsConfig {
  model: string;
  modelType: string;
  client: Client;
}

export class EmbeddingModel {
  private runClient: RunClient;

  constructor(private readonly config: EmbeddingsConfig) {
    this.config = config;
    this.runClient = new RunClient(config.client);
  }

  async embed(query: string): Promise<number[]> {
    switch (this.config.modelType) {
      case "openai":
        return this.openAIEmbed(query);
      default:
        return this.openAIEmbed(query);
    }
  }

  handleError(error: HTTPError) {
    const { model } = this.config;
    const message = `Error embedding request with model ${model} -> ${error.status_code} ${error.message}`;
    return new HTTPError(error.status_code, message);
  }

  async openAIEmbed(query: string): Promise<number[]> {
    try {
      const { model } = this.config;
      const data = (await this.runClient.run({
        resourceType: "model",
        resourceName: model,
        method: "POST",
        json: { input: query },
        path: "/v1/embeddings",
      })) as { data: [{ embedding: number[] }] };
      return data.data[0].embedding;
    } catch (error: any) {
      if (error instanceof HTTPError) {
        throw this.handleError(error);
      }
      throw error;
    }
  }
}
