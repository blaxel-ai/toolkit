from blaxel.aimon.settings import init
from blaxel.api.models import list_models
from blaxel.authentication import new_client
from blaxel.deploy import generate_blaxel_deployment

settings = init()
client = new_client()
models = list_models.sync(client=client)

print(models)
generate_blaxel_deployment(".blaxel", "")