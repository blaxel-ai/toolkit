/* eslint-disable @typescript-eslint/no-require-imports */
import { client } from "../client/sdk.gen.js";
import { Knowledgebase } from "../client/types.gen.js";
import { getSettings } from "../common/settings.js";
import { EmbeddingModel } from "./embeddings.js";
import { KnowledgebaseClass, KnowledgebaseSearchResult } from "./types.js";

export class QdrantKnowledgebase implements KnowledgebaseClass {
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
    const { QdrantClient } = require("@qdrant/js-client-rest");
    const settings = getSettings();
    this.config = connection.config || {};
    this.secrets = connection.secrets || {};
    this.client = new QdrantClient({
      url: this.config.url || "http://localhost:6333",
      apiKey: this.secrets.apiKey || "",
      checkCompatibility: false,
    });
    this.collectionName = this.config.collectionName || settings.name;
    this.scoreThreshold = this.config.scoreThreshold || 0.25;
    this.limit = this.config.limit || 5;
    this.embeddingModel = new EmbeddingModel({
      model: knowledgeBase.spec?.embeddingModel || "",
      modelType: knowledgeBase.spec?.embeddingModelType || "",
      client,
    });
  }

  handleError(action: string, error: Error) {
    if (error instanceof Error && "status" in error) {
      if (
        "data" in error &&
        typeof error.data === "object" &&
        error.data &&
        "status" in error.data &&
        typeof error.data.status === "object" &&
        error.data.status &&
        "error" in error.data.status
      ) {
        return new Error(
          `Qdrant http error for ${action}: ${error.status} - ${error.data.status.error}`
        );
      } else {
        return new Error(
          `Qdrant http error for ${action}: ${error.status} - ${error.message}`
        );
      }
    }
    return error;
  }

  async getOrCreateCollection(embeddings: {
    size: number;
    distance: string;
    retry?: boolean;
  }): Promise<void> {
    try {
      if (embeddings.retry === undefined) {
        embeddings.retry = true;
      }
      const response = await this.client.getCollections();
      if (
        !response.collections.find(
          (collection: any) => collection.name === this.config.collectionName
        )
      ) {
        await this.client.createCollection(this.config.collectionName, {
          vectors: {
            default: {
              size: embeddings.size,
              distance: embeddings.distance,
            },
          },
        });
      }
    } catch (error: any) {
      const message = error.toString().toLowerCase();
      if (
        !embeddings.retry &&
        (message.includes("conflict") || message.includes("already exists"))
      ) {
        return this.getOrCreateCollection({ ...embeddings, retry: false });
      }
      throw this.handleError("creating collection", error as Error);
    }
  }

  async add(key: string, value: string, infos?: any): Promise<void> {
    try {
      const embedding = await this.embeddingModel.embed(value);
      await this.getOrCreateCollection({
        size: embedding.length,
        distance: infos?.distance || "Cosine",
      });
      await this.client.upsert(this.collectionName, {
        points: [
          {
            id: key,
            vector: {
              default: embedding,
            },
            payload: {
              text: value,
              ...infos,
            },
          },
        ],
      });
    } catch (error) {
      throw this.handleError("adding", error as Error);
    }
  }

  async search(
    query: string,
    filters?: any,
    scoreThreshold?: number,
    limit?: number
  ): Promise<Array<KnowledgebaseSearchResult>> {
    try {
      const embedding = await this.embeddingModel.embed(query);
      const results = await this.client.query(this.collectionName, {
        query: embedding,
        using: "default",
        with_payload: true,
        score_threshold: scoreThreshold || this.scoreThreshold,
        limit: limit || this.limit,
      });
      return results.points.map((point: any) => {
        return {
          key: point.id,
          value: JSON.stringify(point.payload),
          similarity: point.score,
        };
      });
    } catch (error) {
      throw this.handleError("searching", error as Error);
    }
  }

  async delete(key: string): Promise<void> {
    try {
      await this.client.delete(this.collectionName, {
        points: [key],
      });
    } catch (error) {
      throw this.handleError("deleting", error as Error);
    }
  }
}
