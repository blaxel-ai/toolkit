from logging import getLogger
from typing import Tuple, Union

from langchain_core.language_models import BaseChatModel

from blaxel.api.models import get_model
from blaxel.authentication import get_authentication_headers, new_client
from blaxel.aimon.settings import get_settings
from blaxel.models import Model

from .voice.openai import OpenAIVoiceReactAgent

logger = getLogger(__name__)


def get_base_url(name: str) -> str:
    """
    Constructs the base URL for a given model name based on the current settings.

    Parameters:
        name (str): The name of the model.

    Returns:
        str: The constructed base URL.
    """
    settings = get_settings()
    return f"{settings.run_url}/{settings.workspace}/models/{name}/v1"


def get_mistral_chat_model(**kwargs) -> BaseChatModel:
    """
    Initializes and returns a MistralAI chat model with the provided keyword arguments.

    Parameters:
        **kwargs: Arbitrary keyword arguments for configuring the `ChatMistralAI` model.

    Returns:
        ChatMistralAI: An instance of the MistralAI chat model.
    """
    from langchain_mistralai.chat_models import ChatMistralAI  # type: ignore

    return ChatMistralAI(**kwargs)


def get_openai_chat_model(**kwargs) -> BaseChatModel:
    """
    Initializes and returns an OpenAI chat model with the provided keyword arguments.

    Parameters:
        **kwargs: Arbitrary keyword arguments for configuring the `ChatOpenAI` model.

    Returns:
        ChatOpenAI: An instance of the OpenAI chat model.
    """
    from langchain_openai import ChatOpenAI  # type: ignore

    return ChatOpenAI(**kwargs)


def get_anthropic_chat_model(**kwargs) -> BaseChatModel:
    """
    Initializes and returns an Anthropic chat model with the provided keyword arguments.

    Parameters:
        **kwargs: Arbitrary keyword arguments for configuring the `ChatAnthropic` model.

    Returns:
        ChatAnthropic: An instance of the Anthropic chat model.
    """
    from langchain_anthropic import ChatAnthropic  # type: ignore

    return ChatAnthropic(**kwargs)


def get_xai_chat_model(**kwargs) -> BaseChatModel:
    """
    Initializes and returns an XAI chat model with the provided keyword arguments.

    Parameters:
        **kwargs: Arbitrary keyword arguments for configuring the `ChatXAI` model.

    Returns:
        ChatXAI: An instance of the XAI chat model.
    """
    from langchain_xai import ChatXAI  # type: ignore

    return ChatXAI(**kwargs)


def get_cohere_chat_model(**kwargs) -> BaseChatModel:
    """
    Initializes and returns a Cohere chat model with the provided keyword arguments.

    Parameters:
        **kwargs: Arbitrary keyword arguments for configuring the `ChatCohere` model.

    Returns:
        ChatCohere: An instance of the Cohere chat model.
    """
    from langchain_cohere import ChatCohere  # type: ignore

    return ChatCohere(**kwargs)

def get_deepseek_chat_model(**kwargs):
    from langchain_deepseek import ChatDeepSeek  # type: ignore

    return ChatDeepSeek(**kwargs)

def get_azure_ai_inference_chat_model(**kwargs):
    from langchain_openai import ChatOpenAI  # type: ignore

    return ChatOpenAI(
        **kwargs
    )  # It uses a compatible endpoint, so we can use the ChatOpenAI interface

def get_azure_marketplace_chat_model(**kwargs):
    from langchain_openai import OpenAI  # type: ignore

    return OpenAI(
        **kwargs
    )  # It seems to use a compatible endpoint, so we can use the classic OpenAI interface

def get_gemini_chat_model(**kwargs):
    from langchain_google_genai import ChatGoogleGenerativeAI  # type: ignore

    return ChatGoogleGenerativeAI(
        **kwargs,
    )

def get_chat_model(name: str, agent_model: Union[Model, None] = None) -> BaseChatModel:
    """
    Gets a chat model instance for the specified model name.

    Parameters:
        name (str): The name of the model to retrieve.
        agent_model (Union[Model, None], optional): A pre-fetched model instance.
            If None, the model will be fetched from the API. Defaults to None.

    Returns:
        BaseChatModel: An instance of the appropriate chat model.
    """
    [chat_model, _, __] = get_chat_model_full(name, agent_model)
    return chat_model

def get_chat_model_full(name: str, agent_model: Union[Model, None] = None) -> Tuple[BaseChatModel, str, str]:
    """
    Gets a chat model instance along with provider and model information.

    Parameters:
        name (str): The name of the model to retrieve.
        agent_model (Union[Model, None], optional): A pre-fetched model instance.
            If None, the model will be fetched from the API. Defaults to None.

    Returns:
        Tuple[BaseChatModel, str, str]: A tuple containing:
            - The chat model instance
            - The provider name (e.g., 'openai', 'anthropic', etc.)
            - The specific model name (e.g., 'gpt-4o-mini')
    """
    settings = get_settings()
    client = new_client()

    if agent_model is None:
        try:
            agent_model = get_model.sync(name, client=client)
        except Exception:
            logger.warning(f"Model {name} not found, defaulting to gpt-4o-mini")

    headers = get_authentication_headers(settings)

    jwt = headers.get("X-Blaxel-Authorization", "").replace("Bearer ", "")
    chat_classes = {
        "openai": {
            "func": get_openai_chat_model,
            "kwargs": {
                "http_async_client": client.get_async_httpx_client(),
                "http_client": client.get_httpx_client(),
            },
        },
        "anthropic": {
            "func": get_anthropic_chat_model,
            "kwargs": {
                "base_url": get_base_url(name).replace("/v1", ""),
            },
            "remove_kwargs": ["default_query"]
        },
        "mistral": {
            "func": get_mistral_chat_model,
            "kwargs": {
                "api_key": jwt,
            },
        },
        "xai": {
            "func": get_xai_chat_model,
            "kwargs": {
                "api_key": jwt,
                "xai_api_base": get_base_url(name),
            },
            "remove_kwargs": ["base_url"],
        },
        "cohere": {
            "func": get_cohere_chat_model,
            "kwargs": {
                "cohere_api_key": jwt,
                "base_url": get_base_url(name).replace("/v1", ""),
            },
        },
        "deepseek": {
            "func": get_deepseek_chat_model,
            "kwargs": {
                "api_key": jwt,
                "api_base": get_base_url(name),
            },
        },
        "azure-ai-inference": {
            "func": get_azure_ai_inference_chat_model,
            "kwargs": {
                "base_url": get_base_url(name).replace("/v1", ""),
            },
        },
        "azure-marketplace": {
            "func": get_azure_marketplace_chat_model,
            "kwargs": {},
        },
        "gemini": {
            "func": get_gemini_chat_model,
            "kwargs": {
                "api_key": "fake_api_key",
                "client_options": {
                    "api_endpoint": get_base_url(name).replace("/v1", "")
                },
                "transport": "rest",
                "additional_headers": {"X-Blaxel-Authorization": f"Bearer {jwt}"},
            },
            "remove_kwargs": ["api_key", "default_headers"]
        },
    }

    provider = (
        agent_model
        and agent_model.spec
        and agent_model.spec.runtime
        and agent_model.spec.runtime.type_
    )
    if not provider:
        logger.warning("Provider not found in agent model, defaulting to OpenAI")
        provider = "openai"

    model = (
        agent_model
        and agent_model.spec
        and agent_model.spec.runtime
        and agent_model.spec.runtime.model
    )
    if not model:
        logger.warning("Model not found in agent model, defaulting to gpt-4o-mini")
        model = "gpt-4o-mini"

    if provider == "openai" and "realtime" in model:
        logger.info("Starting OpenAI Realtime Agent")
        return (
            OpenAIVoiceReactAgent(
                url=get_base_url(name),
                model=model,
                headers=headers
            ),
            provider,
            model
        )
    kwargs = {
        "model": model,
        "base_url": get_base_url(name),
        "default_headers": headers,
        "api_key": "fake_api_key",
        "temperature": 0,
    }
    chat_class = chat_classes.get(provider)
    if not chat_class:
        logger.warning(f"Provider {provider} not currently supported, defaulting to OpenAI")
        chat_class = chat_classes["openai"]
    if "kwargs" in chat_class:
        kwargs.update(chat_class["kwargs"])
    if "remove_kwargs" in chat_class:
        for key in chat_class["remove_kwargs"]:
            kwargs.pop(key, None)
    return chat_class["func"](**kwargs), provider, model
