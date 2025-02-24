/* eslint-disable @typescript-eslint/no-require-imports */
import { client } from "../client/sdk.gen.js";
import { Knowledgebase } from "../client/types.gen.js";
import { getSettings } from "../common/settings.js";
import { EmbeddingModel } from "./embeddings.js";
import { KnowledgebaseClass, KnowledgebaseSearchResult } from "./types.js";

export class PineconeKnowledgebase implements KnowledgebaseClass {
  private client: any;
  private index: any;
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
    const { Pinecone } = require("@pinecone-database/pinecone");
    const settings = getSettings();
    this.config = connection.config || {};
    this.secrets = connection.secrets || {};

    this.client = new Pinecone({ apiKey: this.secrets.apiKey });
    this.collectionName = this.config.collectionName || settings.name;
    this.scoreThreshold = this.config.scoreThreshold || 0.25;
    this.limit = this.config.limit || 5;
    this.embeddingModel = new EmbeddingModel({
      model: knowledgeBase.spec?.embeddingModel || "",
      modelType: knowledgeBase.spec?.embeddingModelType || "",
      client,
    });
    this.index = this.client.index(
      this.config.indexName,
      this.config.indexHost
    );
  }

  async add(key: string, value: string, infos?: any): Promise<void> {
    const embedding = await this.embeddingModel.embed(value);

    await this.index.namespace(this.collectionName).upsert([
      {
        id: key,
        values: embedding,
        metadata: { ...infos, value, name: "test" },
      },
    ]);
  }

  async search(
    query: string,
    filters?: any,
    scoreThreshold?: number,
    limit?: number
  ): Promise<Array<KnowledgebaseSearchResult>> {
    const embedding = await this.embeddingModel.embed(query);
    const result = await this.index.namespace(this.collectionName).query({
      vector: embedding,
      topK: limit || this.limit,
      includeValues: true,
      includeMetadata: true,
    });

    const results: Array<KnowledgebaseSearchResult> = [];
    result.matches.forEach((match: any) => {
      results.push({
        key: match.id,
        value: match.metadata.value,
        similarity: match.score,
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
