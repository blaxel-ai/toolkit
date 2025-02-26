from typing import List

from blaxel.client import Client

from ..common.error import HTTPError
from ..run import RunClient


class EmbeddingModel:
    def __init__(self, model: str, model_type: str, client: Client):
        self.model = model
        self.model_type = model_type
        self.client = client
        self.run_client = RunClient(client)

    async def embed(self, query: str) -> List[float]:
        if self.model_type == "openai":
            return await self.openai_embed(query)
        return await self.openai_embed(query)  # Default to OpenAI

    def handle_error(self, error: HTTPError) -> HTTPError:
        model = self.model
        message = f"Error embedding request with model {model} -> {error.status_code} {error.message}"
        return HTTPError(error.status_code, message)

    async def openai_embed(self, query: str) -> List[float]:
        try:
            response = self.run_client.run(
                resource_type="model",
                resource_name=self.model,
                method="POST",
                json={"input": query},
                path="/v1/embeddings"
            )
            data = response.json()
            return data["data"][0]["embedding"]
        except HTTPError as error:
            raise self.handle_error(error)
        except Exception as error:
            raise error