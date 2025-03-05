from unittest.mock import AsyncMock, patch

import pytest

from blaxel.knowledgebases import KnowledgebaseConfig, KnowledgebaseFactory
from blaxel.knowledgebases.types import KnowledgebaseSearchResult


@pytest.mark.asyncio
async def test_knowledgebase_operations():
    # Mock configurations
    test_config = KnowledgebaseConfig(
        type="chroma",  # We'll test with one implementation
        knowledge_base={
            "spec": {
                "embedding_model": "text-embedding-ada-002",
                "embedding_model_type": "openai"
            }
        },
        connection={
            "config": {
                "url": "http://mock-url",
                "collection_name": "test_collection"
            }
        }
    )

    # Mock search results
    mock_results = [
        KnowledgebaseSearchResult(
            key="doc2",
            value="Machine learning is a subset of artificial intelligence",
            similarity=0.85
        )
    ]

    # Setup mocks
    with patch("chromadb.HttpClient") as mock_chroma, \
         patch("blaxel.knowledgebases.embeddings.EmbeddingModel") as mock_embedding:

        # Configure embedding mock
        mock_embedding.return_value.embed = AsyncMock(
            return_value=[0.1, 0.2, 0.3]  # Mock embedding vector
        )

        # Configure Chroma client mock
        mock_collection = AsyncMock()
        mock_collection.add = AsyncMock()
        mock_collection.query = AsyncMock(return_value={
            "ids": [["doc2"]],
            "distances": [[0.85]],
            "documents": [["Machine learning is a subset of artificial intelligence"]]
        })
        mock_chroma.return_value.get_or_create_collection = AsyncMock(
            return_value=mock_collection
        )
        mock_chroma.return_value.delete = AsyncMock()

        # Create knowledgebase instance
        kb = await KnowledgebaseFactory.create(test_config)

        # Test add operation
        await kb.add(
            key="doc1",
            value="Test document",
            infos={"category": "test"}
        )
        mock_embedding.return_value.embed.assert_called_once_with("Test document")
        mock_collection.add.assert_called_once()

        # Test search operation
        results = await kb.search(
            query="AI and ML",
            score_threshold=0.7,
            limit=5
        )
        assert len(results) == 1
        assert results[0].key == mock_results[0].key
        assert results[0].value == mock_results[0].value
        assert results[0].similarity == mock_results[0].similarity

        # Test delete operation
        await kb.delete("doc1")
        mock_chroma.return_value.delete.assert_called_once()

@pytest.mark.asyncio
async def test_factory_invalid_type():
    invalid_config = KnowledgebaseConfig(
        type="invalid",
        knowledge_base={},
        connection={}
    )

    with pytest.raises(ValueError) as exc_info:
        await KnowledgebaseFactory.create(invalid_config)
    assert "Unsupported memory store type: invalid" in str(exc_info.value)