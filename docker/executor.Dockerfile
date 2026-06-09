FROM ghcr.io/astral-sh/uv:0.11.19-python3.12-trixie-slim

WORKDIR /app
ENV PATH="/app/.venv/bin:${PATH}"

COPY llm_serve/pyproject.toml llm_serve/uv.lock llm_serve/README.md ./
RUN uv sync --frozen --no-dev

COPY llm_serve/ ./

EXPOSE 19991
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "19991"]
