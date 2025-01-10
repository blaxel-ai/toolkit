from beamlit.functions import function


@function(
    function={
        "spec": {
            "policies": ["only-us"],
        }
    }
)
def helloworld(query: str):
    """A function for saying hello to the world."""
    return "Hello from Beamlit!"
