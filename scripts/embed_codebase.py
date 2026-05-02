"""
embed_codebase.py
=================
Ingests the Thaw codebase (.go, .ts, .tsx), chunks it with language-aware
splitters, embeds with Google Gemini (gemini-embedding-2), and persists
everything to a local ChromaDB collection.

NOTE: The google-genai SDK routes gemini-embedding-2 list inputs as a single
multi-part content, producing one embedding.  We work around this by issuing
one embed_content call per chunk, parallelised with asyncio + a semaphore so
we stay well inside the 1 500 RPM free-tier limit.

Run from the scripts/ directory:
    GEMINI_API_KEY=... python embed_codebase.py

Or with a fresh rebuild:
    GEMINI_API_KEY=... python embed_codebase.py --reset
"""

import argparse
import asyncio
import os
import sys
import uuid
from pathlib import Path
from typing import Generator

import chromadb
from chromadb.config import Settings
from langchain_text_splitters import Language, RecursiveCharacterTextSplitter
from google import genai
from google.genai import types
from google.genai import errors as genai_errors
import logging

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s  %(levelname)-8s  %(message)s",
    datefmt="%H:%M:%S",
)
log = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------
SCRIPT_DIR = Path(__file__).resolve().parent
REPO_ROOT = SCRIPT_DIR.parent
CHROMA_DIR = REPO_ROOT / ".chroma_db"

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
COLLECTION_NAME = "thaw_codebase"

GEMINI_EMBED_MODEL = "models/gemini-embedding-2"
GEMINI_EMBED_DIMS = 768      # Matryoshka: 64–3 072; 768 keeps the index compact

# Free tier: 1 500 RPM  →  25 req/s.  We cap at 20 to leave headroom.
CONCURRENCY = 20
MAX_RETRIES = 8              # per-chunk retry attempts before aborting

CHROMA_ADD_BATCH_SIZE = 500  # documents per chroma .add() call

CHUNK_SIZE = 1_500           # characters (~375 tokens)
CHUNK_OVERLAP = 150

INCLUDE_EXTENSIONS = {".go", ".ts", ".tsx"}

IGNORE_DIRS = {
    "vendor",
    "node_modules",
    "dist",
    "build",
    "out",
    ".next",
    ".git",
    ".idea",
    ".vscode",
    ".chroma_db",
    "wailsjs",   # auto-generated bindings — no business logic
}

# ---------------------------------------------------------------------------
# Language-aware splitters
# ---------------------------------------------------------------------------
_go_splitter = RecursiveCharacterTextSplitter.from_language(
    language=Language.GO,
    chunk_size=CHUNK_SIZE,
    chunk_overlap=CHUNK_OVERLAP,
)
_ts_splitter = RecursiveCharacterTextSplitter.from_language(
    language=Language.TS,
    chunk_size=CHUNK_SIZE,
    chunk_overlap=CHUNK_OVERLAP,
)


def get_splitter(ext: str) -> RecursiveCharacterTextSplitter:
    return _go_splitter if ext == ".go" else _ts_splitter


# ---------------------------------------------------------------------------
# File walking
# ---------------------------------------------------------------------------
def walk_source_files(root: Path) -> Generator[Path, None, None]:
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in IGNORE_DIRS]
        for filename in filenames:
            p = Path(dirpath) / filename
            if p.suffix in INCLUDE_EXTENSIONS:
                yield p


# ---------------------------------------------------------------------------
# Chunking
# ---------------------------------------------------------------------------
def chunk_file(path: Path) -> list[dict]:
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError as exc:
        log.warning("Skipping %s — could not read: %s", path, exc)
        return []

    if not text.strip():
        return []

    rel_path = path.relative_to(REPO_ROOT).as_posix()
    lang = "go" if path.suffix == ".go" else "typescript"
    splitter = get_splitter(path.suffix)

    result = []
    for i, chunk in enumerate(splitter.split_text(text)):
        if not chunk.strip():
            continue
        result.append(
            {
                "id": str(uuid.uuid4()),
                "text": chunk,
                "metadata": {"file_path": rel_path, "language": lang, "chunk_index": i},
            }
        )
    return result


# ---------------------------------------------------------------------------
# Async embedding  (one call per chunk, parallelised with a semaphore)
# ---------------------------------------------------------------------------
async def _embed_one(
    sem: asyncio.Semaphore,
    aclient: genai.Client,
    text: str,
    progress: dict,
) -> list[float]:
    """Embed a single text with exponential back-off retry."""
    async with sem:
        for attempt in range(MAX_RETRIES):
            try:
                result = await aclient.aio.models.embed_content(
                    model=GEMINI_EMBED_MODEL,
                    contents=text,
                    config=types.EmbedContentConfig(
                        task_type="RETRIEVAL_DOCUMENT",
                        output_dimensionality=GEMINI_EMBED_DIMS,
                    ),
                )
                # Single-string input → always exactly one embedding
                vec = result.embeddings[0].values

                progress["done"] += 1
                n = progress["done"]
                total = progress["total"]
                if n % 100 == 0 or n == total:
                    log.info("  Embedded %d / %d chunks (%.0f%%)", n, total, 100 * n / total)

                return vec

            except (genai_errors.ClientError, genai_errors.ServerError) as exc:
                wait = min(4 * (2 ** attempt), 60)
                log.warning(
                    "Embed attempt %d/%d failed (%s). Retrying in %.0fs…",
                    attempt + 1, MAX_RETRIES, exc, wait,
                )
                await asyncio.sleep(wait)

        raise RuntimeError(f"Chunk embedding failed after {MAX_RETRIES} attempts")


async def embed_all(chunks: list[dict]) -> list[list[float]]:
    """Embed every chunk concurrently (bounded by CONCURRENCY semaphore)."""
    aclient = genai.Client(api_key=os.environ["GEMINI_API_KEY"])
    sem = asyncio.Semaphore(CONCURRENCY)
    progress = {"done": 0, "total": len(chunks)}

    tasks = [
        _embed_one(sem, aclient, chunk["text"], progress)
        for chunk in chunks
    ]
    # gather preserves order
    return list(await asyncio.gather(*tasks))


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
def batched(lst: list, size: int) -> Generator[list, None, None]:
    for i in range(0, len(lst), size):
        yield lst[i : i + size]


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
def main(reset: bool = False) -> None:
    log.info("Repository root : %s", REPO_ROOT)
    log.info("ChromaDB dir    : %s", CHROMA_DIR)
    log.info("Embed model     : %s  (%d dims)", GEMINI_EMBED_MODEL, GEMINI_EMBED_DIMS)
    log.info("Concurrency     : %d parallel embed calls", CONCURRENCY)

    # ------------------------------------------------------------------ Chroma
    chroma_client = chromadb.PersistentClient(
        path=str(CHROMA_DIR),
        settings=Settings(anonymized_telemetry=False),
    )

    if reset:
        try:
            chroma_client.delete_collection(COLLECTION_NAME)
            log.info("Deleted existing collection '%s'.", COLLECTION_NAME)
        except Exception:
            pass

    collection = chroma_client.get_or_create_collection(
        name=COLLECTION_NAME,
        metadata={"hnsw:space": "cosine"},
    )
    log.info("Collection '%s' — %d existing documents.", COLLECTION_NAME, collection.count())

    # --------------------------------------------------------- Walk & chunk
    log.info("Scanning source files…")
    all_chunks: list[dict] = []
    file_count = 0

    for path in walk_source_files(REPO_ROOT):
        chunks = chunk_file(path)
        if chunks:
            all_chunks.extend(chunks)
            file_count += 1

    total = len(all_chunks)
    log.info("Found %d files → %d chunks to embed.", file_count, total)

    if not all_chunks:
        log.warning("No chunks produced. Exiting.")
        return

    # ------------------------------------------------------ Embed (async)
    log.info("Embedding %d chunks (concurrency=%d)…", total, CONCURRENCY)
    vectors: list[list[float]] = asyncio.run(embed_all(all_chunks))

    # --------------------------------------------------- Store in ChromaDB
    log.info("Storing in ChromaDB…")
    stored = 0

    for batch in batched(list(zip(all_chunks, vectors)), CHROMA_ADD_BATCH_SIZE):
        chunks_b, vectors_b = zip(*batch)
        collection.add(
            ids=[c["id"] for c in chunks_b],
            embeddings=list(vectors_b),
            documents=[c["text"] for c in chunks_b],
            metadatas=[c["metadata"] for c in chunks_b],
        )
        stored += len(chunks_b)
        log.info("  Stored %d / %d chunks.", stored, total)

    log.info("Done. %d chunks stored in '%s'.", stored, CHROMA_DIR)
    log.info("Total documents in collection: %d", collection.count())


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------
if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="Embed the Thaw codebase into a local ChromaDB instance."
    )
    parser.add_argument(
        "--reset",
        action="store_true",
        help="Delete and rebuild the collection from scratch.",
    )
    args = parser.parse_args()

    if not os.environ.get("GEMINI_API_KEY"):
        log.error("GEMINI_API_KEY environment variable is not set.")
        sys.exit(1)

    main(reset=args.reset)
