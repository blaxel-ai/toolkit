export type KnowledgebaseSearchResult = {
  key: string;
  value: string;
  similarity: number;
};
export interface KnowledgebaseClass {
  add(key: string, value: string, infos?: any): Promise<void>;
  search(
    query: string,
    filters?: any,
    scoreThreshold?: number,
    limit?: number
  ): Promise<Array<KnowledgebaseSearchResult>>;
  delete(key: string): Promise<void>;
}
