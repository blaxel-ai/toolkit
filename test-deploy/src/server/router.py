import uuid

from blaxel.instrumentation.span import SpanManager
from fastapi import APIRouter, Request
from fastapi.responses import StreamingResponse

from ..agent import agent

router = APIRouter()

@router.post("/")
async def handle_request(request: Request):
    user_id = request.headers.get("X-User-Id", str(uuid.uuid4()))
    session_id = request.headers.get("X-Session-Id", str(uuid.uuid4()))
    body = await request.json()
    with SpanManager("blaxel-google-adk").create_active_span("agent-request", {"user_id": user_id, "session_id": session_id}):
        return StreamingResponse(
            agent(body["inputs"], user_id, session_id),
            media_type='text/event-stream'
        )