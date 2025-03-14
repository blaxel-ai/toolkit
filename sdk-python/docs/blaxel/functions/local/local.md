Module blaxel.functions.local.local
===================================

Classes
-------

`LocalToolKit(client: blaxel.client.AuthenticatedClient, local_function: dict)`
:   Toolkit for managing local tools.
    
    Attributes:
        client (AuthenticatedClient): The authenticated client instance.
        function (str): The name of the local function to integrate.
        _function (Function | None): Cached Function object after initialization.

    ### Class variables

    `client: blaxel.client.AuthenticatedClient`
    :

    `local_function: dict`
    :

    `model_config`
    :

    ### Methods

    `get_tools(self) ‑> list[langchain_core.tools.base.BaseTool]`
    :

    `initialize(self) ‑> None`
    :   Initialize the session and retrieve the local function details.