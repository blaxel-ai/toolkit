import os
import uuid
from typing import List

from blaxel.common import init
from blaxel.knowledgebases import (
    KnowledgebaseClass,
    KnowledgebaseConfig,
    KnowledgebaseFactory,
    KnowledgebaseSearchResult,
)

settings = init()

async def sample_usage():
    # Sample configuration for Chroma
    chroma_config = KnowledgebaseConfig(
        type="chroma",
        knowledge_base={
            "spec": {
                "embeddingModel": "text-embedding-3-large",
                "embeddingModelType": "openai"
            }
        },
        connection={
            "config": {
                "url": "http://localhost:8000",
                "collectionName": "my_collection",
            }
        }
    )

    # Sample configuration for Pinecone
    pinecone_config = KnowledgebaseConfig(
        type="pinecone",
        knowledge_base={
            "spec": {
                "embeddingModel": "text-embedding-3-large",
                "embeddingModelType": "openai"
            }
        },
        connection={
            "config": {
                "indexName": "beamlit-test-3072",
                "indexHost": "https://beamlit-test-3072-sll9m6p.svc.aped-4627-b74a.pinecone.io"
            },
            "secrets": {
                "apiKey": os.getenv("PINECONE_API_KEY")
            }
        }
    )

    # Sample configuration for Qdrant
    qdrant_config = KnowledgebaseConfig(
        type="qdrant",
        knowledge_base={
            "spec": {
                "embeddingModel": "text-embedding-3-large",
                "embeddingModelType": "openai"
            }
        },
        connection={
            "config": {
                "url": os.getenv("QDRANT_URL"),
                "collectionName": "my_collection"
            },
            "secrets": {
                "apiKey": os.getenv("QDRANT_API_KEY")
            }
        }
    )

    # Create knowledgebase instances
    chroma_kb = await KnowledgebaseFactory.create(chroma_config)
    pinecone_kb = await KnowledgebaseFactory.create(pinecone_config)
    qdrant_kb = await KnowledgebaseFactory.create(qdrant_config)

    # Example usage with any knowledgebase implementation
    async def demonstrate_kb_operations(kb: KnowledgebaseClass):
        # Add some documents
        id1 = str(uuid.uuid4())
        id2 = str(uuid.uuid4())
        await kb.add(
            key=id1,
            value="The quick brown fox jumps over the lazy dog",
            infos={"category": "sample", "language": "en", "name": "doc1"}
        )

        await kb.add(
            key=id2,
            value="Machine learning is a subset of artificial intelligence",
            infos={"category": "tech", "language": "en"}
        )

        # Search for similar documents
        results: List[KnowledgebaseSearchResult] = await kb.search(
            query="Tell me about AI",
            score_threshold=0.2,
            limit=5
        )

        # Print results
        for result in results:
            print(f"Key: {result.key}")
            print(f"Value: {result.value}")
            print(f"Similarity: {result.similarity}")
            print("---")

        # Delete a document
        await kb.delete(id1)
        await kb.delete(id2)

        await kb.close()

    # Try with different implementations
    print("Testing with Chroma:")
    await demonstrate_kb_operations(chroma_kb)

    print("\nTesting with Pinecone:")
    await demonstrate_kb_operations(pinecone_kb)

    print("\nTesting with Qdrant:")
    await demonstrate_kb_operations(qdrant_kb)

# Run the sample
if __name__ == "__main__":
    import asyncio
    asyncio.run(sample_usage())