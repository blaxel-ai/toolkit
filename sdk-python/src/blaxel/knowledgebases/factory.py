from typing import Any, Dict

from .chroma import ChromaKnowledgebase
from .pinecone import PineconeKnowledgebase
from .qdrant import QdrantKnowledgebase
from .types import KnowledgebaseClass


class KnowledgebaseConfig:
    def __init__(
        self,
        type: str,
        knowledge_base: Dict[str, Any],
        connection: Dict[str, Any]
    ):
        self.type = type
        self.knowledge_base = knowledge_base
        self.connection = connection

class KnowledgebaseFactory:
    @staticmethod
    async def create(config: KnowledgebaseConfig) -> KnowledgebaseClass:
        try:
            if config.type == "qdrant":
                return QdrantKnowledgebase(config.connection, config.knowledge_base)
            elif config.type == "chroma":
                return ChromaKnowledgebase(config.connection, config.knowledge_base)
            elif config.type == "pinecone":
                return PineconeKnowledgebase(config.connection, config.knowledge_base)
            else:
                raise ValueError(f"Unsupported memory store type: {config.type}")
        except Exception as e:
            raise ValueError(f"Error creating knowledgebase: {e}") from e
