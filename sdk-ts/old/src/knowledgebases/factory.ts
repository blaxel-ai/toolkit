// import { PostgresKnowledgebase } from "./postgres";
import { Knowledgebase } from "../client/types.gen.js";
import { ChromaKnowledgebase } from "./chroma.js";
import { PineconeKnowledgebase } from "./pinecone.js";
import { QdrantKnowledgebase } from "./qdrant.js";
import { KnowledgebaseClass } from "./types.js";

export type KnowledgebaseConfig = {
  type: string;
  knowledgeBase: Knowledgebase;
  connection: {
    url?: string;
    secrets?: any;
    config?: any;
    // Add other common connection params as needed
  };
};

export class KnowledgebaseFactory {
  static async create(
    config: KnowledgebaseConfig
  ): Promise<KnowledgebaseClass> {
    switch (config.type) {
      case "qdrant":
        return new QdrantKnowledgebase(config.connection, config.knowledgeBase);
      case "chroma":
        return new ChromaKnowledgebase(config.connection, config.knowledgeBase);
      case "pinecone":
        return new PineconeKnowledgebase(
          config.connection,
          config.knowledgeBase
        );
      default:
        throw new Error(`Unsupported memory store type: ${config.type}`);
    }
  }
}
