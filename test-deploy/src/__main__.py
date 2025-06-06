import uvicorn
from blaxel import env

port = env["BL_SERVER_PORT"]
host = env["BL_SERVER_HOST"]

if __name__ == "__main__":
    uvicorn.run("src.main:app", host=host, port=int(port), reload=False)