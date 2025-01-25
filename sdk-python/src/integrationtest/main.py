from beamlit.api.models import list_models
from beamlit.authentication import new_client
from beamlit.common.settings import init
from beamlit.deploy import generate_beamlit_deployment

settings = init()
client = new_client()
models = list_models.sync(client=client)

print(models)
generate_beamlit_deployment(".beamlit", "")