Module blaxel.agents.voice.openai
=================================

Classes
-------

`OpenAIVoiceReactAgent(**data: Any)`
:   .. beta::
       
    
    Create a new model by parsing and validating input data from keyword arguments.
    
    Raises [`ValidationError`][pydantic_core.ValidationError] if the input data cannot be
    validated to form a valid model.
    
    `self` is explicitly positional-only to allow `self` as a field name.

    ### Ancestors (in MRO)

    * pydantic.main.BaseModel

    ### Class variables

    `headers: dict[str, typing.Any]`
    :

    `instructions: str | None`
    :

    `model: str`
    :

    `model_config`
    :

    `tools: list[langchain_core.tools.base.BaseTool] | None`
    :

    `url: str`
    :

    ### Methods

    `aconnect(self, input_stream: AsyncIterator[str], send_output_chunk: Callable[[str], Coroutine[Any, Any, None]]) ‑> None`
    :   Connect to the OpenAI API and send and receive messages.
        
        input_stream: AsyncIterator[str]
            Stream of input events to send to the model. Usually transports input_audio_buffer.append events from the microphone.
        output: Callable[[str], None]
            Callback to receive output events from the model. Usually sends response.audio.delta events to the speaker.

    `bind_tools(self, tools: list[langchain_core.tools.base.BaseTool]) ‑> None`
    :