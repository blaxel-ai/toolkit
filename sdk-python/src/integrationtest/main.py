from beamlit.api.models import get_model, list_models
from beamlit.authentication import get_authentication_headers, new_client
from beamlit.common.settings import init
from beamlit.models import Model
from beamlit.run import RunClient

settings = init()
client = new_client()
models = list_models.sync(client=client)
model: Model = get_model.sync(
    "gpt-3-5-turbo", client=client
)
run_client = RunClient(client=client)
response = run_client.run(
    "model",
    "gpt-3-5-turbo",
    settings.environment,
    method="POST",
    path="/v1/chat/completions",
    json={
        "messages": [
            {
            "role": "user",
            "content": "Hello!"
            }
        ]
    }
)

if response.status_code == 200:
    print(response.json())
else:
    print(response.text)
print(get_authentication_headers(settings))
