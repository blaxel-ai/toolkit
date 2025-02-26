from typing import Any, Dict, List, Optional
from urllib.parse import urlparse

from ..authentication import new_client
from ..common.settings import get_settings
from .embeddings import EmbeddingModel
from .types import KnowledgebaseClass, KnowledgebaseSearchResult


class ChromaKnowledgebase(KnowledgebaseClass):
    def __init__(self, connection: Dict[str, Any], knowledge_base: Dict[str, Any]):
        import chromadb

        settings = get_settings()
        self.config = connection.get("config", {})
        self.secrets = connection.get("secrets", {})

        auth = None
        if self.secrets.get("password") and self.config.get("username"):
            import base64
            credentials = base64.b64encode(
                f"{self.config['username']}:{self.secrets['password']}".encode()
            ).decode()
            auth = {
                "provider": "basic",
                "credentials": credentials
            }
        url = self.config.get("url", "http://localhost:8000")
        # Split URL into host and optional port

        parsed_url = urlparse(url)
        host = parsed_url.hostname
        port = parsed_url.port
        options = {
            "host": host,
            "port": port if port else None
        }
        if auth:
            options["auth"] = auth
        self.options = options
        self.client: Optional[chromadb.AsyncClientAPI] = None
        self.collection_name = self.config.get("collectionName", settings.name)
        self.score_threshold = self.config.get("scoreThreshold", 0.25)
        self.limit = self.config.get("limit", 5)
        self.embedding_model = EmbeddingModel(
            model=knowledge_base.get("spec", {}).get("embeddingModel", ""),
            model_type=knowledge_base.get("spec", {}).get("embeddingModelType", ""),
            client=new_client()
        )

    async def get_client(self):
        import chromadb

        if not self.client:
            self.client = await chromadb.AsyncHttpClient(**self.options)
        return self.client

    async def close(self):
        pass

    async def get_collection(self):
        client = await self.get_client()
        return await client.get_or_create_collection(name=self.collection_name)

    async def add(self, key: str, value: str, infos: Optional[Any] = None) -> None:
        embedding = await self.embedding_model.embed(value)
        collection = await self.get_collection()
        await collection.add(
            ids=[key],
            embeddings=[embedding],
            metadatas=[infos],
            documents=[value]
        )

    async def search(
        self,
        query: str,
        filters: Optional[Any] = None,
        score_threshold: Optional[float] = None,
        limit: Optional[int] = None
    ) -> List[KnowledgebaseSearchResult]:
        collection = await self.get_collection()
        embedding = await self.embedding_model.embed(query)
        result = await collection.query(
            query_embeddings=embedding,
            n_results=limit or self.limit
        )

        results = []
        for doc_index, documents in enumerate(result["ids"]):
            for index, id in enumerate(documents):
                distance = result["distances"][doc_index][index]
                value = result["documents"][doc_index][index]
                if distance >= (score_threshold or self.score_threshold):
                    results.append(KnowledgebaseSearchResult(
                        key=id,
                        value=value,
                        similarity=distance
                    ))

        return results

    async def delete(self, key: str) -> None:
        collection = await self.get_collection()
        await collection.delete(
            ids=[key]
        )

