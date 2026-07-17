FROM ghcr.io/astral-sh/uv:0.11.19-python3.12-trixie-slim

WORKDIR /app
ENV PATH="/app/.venv/bin:${PATH}"

COPY executor/pyproject.toml executor/uv.lock executor/README.md ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-dev

COPY executor/ ./

EXPOSE 19991
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "19991"]
