Module blaxel.agents.chat
=========================

Functions
---------

`get_anthropic_chat_model(**kwargs) ‑> langchain_core.language_models.chat_models.BaseChatModel`
:   Initializes and returns an Anthropic chat model with the provided keyword arguments.
    
    Parameters:
        **kwargs: Arbitrary keyword arguments for configuring the `ChatAnthropic` model.
    
    Returns:
        ChatAnthropic: An instance of the Anthropic chat model.

`get_azure_ai_inference_chat_model(**kwargs)`
:   

`get_azure_marketplace_chat_model(**kwargs)`
:   

`get_base_url(name: str) ‑> str`
:   Constructs the base URL for a given model name based on the current settings.
    
    Parameters:
        name (str): The name of the model.
    
    Returns:
        str: The constructed base URL.

`get_chat_model(name: str, agent_model: blaxel.models.model.Model | None = None) ‑> langchain_core.language_models.chat_models.BaseChatModel`
:   Gets a chat model instance for the specified model name.
    
    Parameters:
        name (str): The name of the model to retrieve.
        agent_model (Union[Model, None], optional): A pre-fetched model instance.
            If None, the model will be fetched from the API. Defaults to None.
    
    Returns:
        BaseChatModel: An instance of the appropriate chat model.

`get_chat_model_full(name: str, agent_model: blaxel.models.model.Model | None = None) ‑> Tuple[langchain_core.language_models.chat_models.BaseChatModel, str, str]`
:   Gets a chat model instance along with provider and model information.
    
    Parameters:
        name (str): The name of the model to retrieve.
        agent_model (Union[Model, None], optional): A pre-fetched model instance.
            If None, the model will be fetched from the API. Defaults to None.
    
    Returns:
        Tuple[BaseChatModel, str, str]: A tuple containing:
            - The chat model instance
            - The provider name (e.g., 'openai', 'anthropic', etc.)
            - The specific model name (e.g., 'gpt-4o-mini')

`get_cohere_chat_model(**kwargs) ‑> langchain_core.language_models.chat_models.BaseChatModel`
:   Initializes and returns a Cohere chat model with the provided keyword arguments.
    
    Parameters:
        **kwargs: Arbitrary keyword arguments for configuring the `ChatCohere` model.
    
    Returns:
        ChatCohere: An instance of the Cohere chat model.

`get_deepseek_chat_model(**kwargs)`
:   

`get_gemini_chat_model(**kwargs)`
:   

`get_mistral_chat_model(**kwargs) ‑> langchain_core.language_models.chat_models.BaseChatModel`
:   Initializes and returns a MistralAI chat model with the provided keyword arguments.
    
    Parameters:
        **kwargs: Arbitrary keyword arguments for configuring the `ChatMistralAI` model.
    
    Returns:
        ChatMistralAI: An instance of the MistralAI chat model.

`get_openai_chat_model(**kwargs) ‑> langchain_core.language_models.chat_models.BaseChatModel`
:   Initializes and returns an OpenAI chat model with the provided keyword arguments.
    
    Parameters:
        **kwargs: Arbitrary keyword arguments for configuring the `ChatOpenAI` model.
    
    Returns:
        ChatOpenAI: An instance of the OpenAI chat model.

`get_xai_chat_model(**kwargs) ‑> langchain_core.language_models.chat_models.BaseChatModel`
:   Initializes and returns an XAI chat model with the provided keyword arguments.
    
    Parameters:
        **kwargs: Arbitrary keyword arguments for configuring the `ChatXAI` model.
    
    Returns:
        ChatXAI: An instance of the XAI chat model.