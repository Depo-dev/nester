import logging
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.routers import analyze, chat, health

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")
logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    logger.info("Intelligence service started")
    yield


app = FastAPI(title="Nester Intelligence", lifespan=lifespan)
app.include_router(health.router)
app.include_router(chat.router, prefix="/intelligence")
app.include_router(analyze.router)
