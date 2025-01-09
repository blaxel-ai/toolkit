from beamlit.functions import function


@function()
def helloworld2(query: str):
    """A function for saying hello to the world."""
    return "Hello from beamlit!"
