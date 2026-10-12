from typing import Literal

from fastapi import APIRouter
from pydantic import BaseModel

router = APIRouter(tags=["health"])


class HealthResponse(BaseModel):
    service: str
    status: Literal["ok"]


@router.get("/healthz", response_model=HealthResponse)
def get_health() -> HealthResponse:
    return HealthResponse(service="cch-ai-engine", status="ok")