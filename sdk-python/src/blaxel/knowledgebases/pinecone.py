from typing import Any, Dict, List, Optional

from ..authentication.authentication import new_client
from ..common.settings import get_settings
from .embeddings import EmbeddingModel
from .types import KnowledgebaseClass, KnowledgebaseSearchResult


class PineconeKnowledgebase(KnowledgebaseClass):
    def __init__(self, connection: Dict[str, Any], knowledge_base: Dict[str, Any]):
        from pinecone import PineconeAsyncio

        settings = get_settings()
        self.config = connection.get("config", {})
        self.secrets = connection.get("secrets", {})

        self.client = PineconeAsyncio(api_key=self.secrets.get("api_key"))
        self.collection_name = self.config.get("collectionName", settings.name)
        self.score_threshold = self.config.get("scoreThreshold", 0.25)
        self.limit = self.config.get("limit", 5)
        self.embedding_model = EmbeddingModel(
            model=knowledge_base.get("spec", {}).get("embeddingModel", ""),
            model_type=knowledge_base.get("spec", {}).get("embeddingModelType", ""),
            client=new_client()
        )
        self.index = self.client.IndexAsyncio(
            host=self.config.get("indexHost")
        )

    async def close(self):
        await self.index.close()
        await self.client.close()

    async def add(self, key: str, value: str, infos: Optional[Any] = None) -> None:
        embedding = await self.embedding_model.embed(value)
        infos = infos if infos else {}
        await self.index.upsert([{
            "id": key,
            "values": embedding,
            "metadata": {
                **infos,
                "value": value,
                "name": "test"
            }
        }], namespace=self.collection_name)

    async def search(
        self,
        query: str,
        filters: Optional[Any] = None,
        score_threshold: Optional[float] = None,
        limit: Optional[int] = None
    ) -> List[KnowledgebaseSearchResult]:
        embedding = await self.embedding_model.embed(query)
        result = await self.index.query(
            vector=embedding,
            top_k=limit or self.limit,
            include_values=True,
            include_metadata=True,
            namespace=self.collection_name
        )

        results = []
        for match in result.matches:
            results.append(KnowledgebaseSearchResult(
                key=match.id,
                value=match.metadata,
                similarity=match.score
            ))
        return results

    async def delete(self, key: str) -> None:
        await self.index.delete(
            namespace=self.collection_name,
            ids=[key]
        )