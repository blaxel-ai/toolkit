from typing import Any, Dict, List, Optional

from ..authentication import new_client
from ..common.settings import get_settings
from .embeddings import EmbeddingModel
from .types import KnowledgebaseClass, KnowledgebaseSearchResult


class QdrantKnowledgebase(KnowledgebaseClass):
    def __init__(self, connection: Dict[str, Any], knowledge_base: Dict[str, Any]):
        from qdrant_client import AsyncQdrantClient, models

        settings = get_settings()

        self.qdrant_models = models
        self.config = connection.get("config", {})
        self.secrets = connection.get("secrets", {})

        self.client = AsyncQdrantClient(
            url=self.config.get("url", "http://localhost:6333"),
            api_key=self.secrets.get("apiKey", ""),
            check_compatibility=False
        )
        self.collection_name = self.config.get("collectionName", settings.name)
        self.score_threshold = self.config.get("scoreThreshold", 0.25)
        self.limit = self.config.get("limit", 5)
        self.embedding_model = EmbeddingModel(
            model=knowledge_base.get("spec", {}).get("embeddingModel", ""),
            model_type=knowledge_base.get("spec", {}).get("embeddingModelType", ""),
            client=new_client()
        )

    async def close(self):
        await self.client.close()

    def handle_error(self, action: str, error: Exception) -> Exception:
        if hasattr(error, "status"):
            if hasattr(error, "data") and isinstance(error.data, dict):
                status = error.data.get("status", {})
                if isinstance(status, dict) and "error" in status:
                    return Exception(
                        f"Qdrant http error for {action}: {error.status} - {status['error']}"
                    )
            return Exception(
                f"Qdrant http error for {action}: {error.status} - {str(error)}"
            )
        return error

    async def get_or_create_collection(
        self,
        embeddings: Dict[str, Any],
        retry: int = 0
    ) -> None:
        try:
            collections = await self.client.get_collections()
            if not any(c.name == self.collection_name for c in collections.collections):
                await self.client.create_collection(
                    collection_name=self.collection_name,
                    vectors_config=self.qdrant_models.VectorParams(
                        size=embeddings["size"],
                        distance=embeddings["distance"]
                    )
                )
        except Exception as error:
            message = str(error).lower()
            if (
                retry < 3 and
                ("conflict" in message or "already exists" in message)
            ):
                return await self.get_or_create_collection(
                    embeddings,
                    retry=retry + 1
                )
            raise self.handle_error("creating collection", error)

    async def add(self, key: str, value: str, infos: Optional[Any] = None) -> None:
        try:
            embedding = await self.embedding_model.embed(value)
            await self.get_or_create_collection({
                "size": len(embedding),
                "distance": infos.get("distance", "Cosine") if infos else "Cosine"
            })

            await self.client.upsert(
                collection_name=self.collection_name,
                points=[self.qdrant_models.PointStruct(
                    id=key,
                    vector=embedding,
                    payload={
                        "text": value,
                        **(infos or {})
                    }
                )]
            )
        except Exception as error:
            raise self.handle_error("adding", error)

    async def search(
        self,
        query: str,
        filters: Optional[Any] = None,
        score_threshold: Optional[float] = None,
        limit: Optional[int] = None
    ) -> List[KnowledgebaseSearchResult]:
        try:
            embedding = await self.embedding_model.embed(query)
            results = await self.client.search(
                collection_name=self.collection_name,
                query_vector=embedding,
                query_filter=filters,
                with_payload=True,
                score_threshold=score_threshold or self.score_threshold,
                limit=limit or self.limit
            )

            return [
                KnowledgebaseSearchResult(
                    key=point.id,
                    value=point.payload,
                    similarity=point.score
                )
                for point in results
            ]
        except Exception as error:
            raise self.handle_error("searching", error)

    async def delete(self, key: str) -> None:
        try:
            await self.client.delete(
                collection_name=self.collection_name,
                points_selector=self.qdrant_models.PointIdsList(
                    points=[key]
                )
            )
        except Exception as error:
            raise self.handle_error("deleting", error)