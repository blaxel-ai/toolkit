/* eslint-disable @typescript-eslint/no-require-imports */
import { client } from "../client/sdk.gen.js";
import { Knowledgebase } from "../client/types.gen.js";
import { getSettings } from "../common/settings.js";
import { EmbeddingModel } from "./embeddings.js";
import { KnowledgebaseClass, KnowledgebaseSearchResult } from "./types.js";

export class ChromaKnowledgebase implements KnowledgebaseClass {
  private client: any;
  private collectionName: string;
  private scoreThreshold: number;
  private limit: number;
  private config: any;
  private secrets: any;
  private embeddingModel: EmbeddingModel;

  constructor(
    connection: {
      collectionName?: string;
      scoreThreshold?: number;
      limit?: number;
      config?: any;
      secrets?: any;
    },
    knowledgeBase: Knowledgebase
  ) {
    const { ChromaClient } = require("chromadb");
    const settings = getSettings();
    this.config = connection.config || {};
    this.secrets = connection.secrets || {};
    const auth =
      this.secrets.password && this.config.username
        ? {
            provider: "basic",
            credentials: Buffer.from(
              `${this.config.username}:${this.secrets.password}`
            ).toString("base64"),
          }
        : null;
    const options: any = {
      url: this.config.url || "http://localhost:8000",
    };
    if (auth) options.auth = auth;
    this.client = new ChromaClient(options);
    this.collectionName = this.config.collectionName || settings.name;
    this.scoreThreshold = this.config.scoreThreshold || 0.25;
    this.limit = this.config.limit || 5;
    this.embeddingModel = new EmbeddingModel({
      model: knowledgeBase.spec?.embeddingModel || "",
      modelType: knowledgeBase.spec?.embeddingModelType || "",
      client,
    });
  }

  async getCollection() {
    return await this.client.getOrCreateCollection({
      name: this.collectionName,
    });
  }

  async add(key: string, value: string, infos?: any): Promise<void> {
    const embedding = await this.embeddingModel.embed(value);
    const collection = await this.getCollection();
    await collection.add({
      ids: [key],
      embeddings: [embedding],
      metadatas: [infos],
      documents: [value],
    });
  }

  async search(
    query: string,
    filters?: any,
    scoreThreshold?: number,
    limit?: number
  ): Promise<Array<KnowledgebaseSearchResult>> {
    const collection = await this.getCollection();
    const embedding = await this.embeddingModel.embed(query);
    const result = await collection.query({
      queryEmbeddings: embedding,
      nResults: limit || this.limit,
    });
    const results: Array<KnowledgebaseSearchResult> = [];

    result.ids.forEach((document: Array<string>, docIndex: number) => {
      document.forEach((id: string, index: number) => {
        const distance = result.distances[docIndex][index];
        const value = result.documents[docIndex][index];
        if (distance >= (scoreThreshold || this.scoreThreshold)) {
          results.push({
            key: id,
            value: value,
            similarity: distance,
          });
        }
      });
    });

    return results;
  }

  async delete(key: string): Promise<void> {
    await this.client.delete(this.collectionName, {
      points: [key],
    });
  }
}
