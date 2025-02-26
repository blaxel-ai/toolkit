from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Any, Dict, List, Optional


@dataclass
class KnowledgebaseSearchResult:
    key: str
    value: Any
    similarity: float


@dataclass
class KnowledgebaseConfig:
    type: str
    knowledge_base: Dict[str, Any]
    connection: Dict[str, Any]


class KnowledgebaseClass(ABC):
    @abstractmethod
    async def close(self) -> None:
        pass

    @abstractmethod
    async def add(self, key: str, value: str, infos: Optional[Any] = None) -> None:
        pass

    @abstractmethod
    async def search(
        self,
        query: str,
        filters: Optional[Any] = None,
        score_threshold: Optional[float] = None,
        limit: Optional[int] = None
    ) -> List[KnowledgebaseSearchResult]:
        pass

    @abstractmethod
    async def delete(self, key: str) -> None:
        pass