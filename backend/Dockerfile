FROM ghcr.io/astral-sh/uv:python3.14-alpine

RUN addgroup nonroot --gid 6666 && \
    adduser nonroot --uid 6666 --ingroup nonroot --disabled-password

WORKDIR /app

ENV UV_COMPILE_BYTECODE=1
ENV UV_LINK_MODE=copy
ENV UV_NO_DEV=1

RUN --mount=type=cache,target=/root/.cache/uv \
    --mount=type=bind,source=uv.lock,target=uv.lock \
    --mount=type=bind,source=pyproject.toml,target=pyproject.toml \
    uv sync --locked --no-install-project

COPY . /app

RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --locked && \
    chown -R nonroot:nonroot /app

ENV PATH="/app/.venv/bin:$PATH"
ENTRYPOINT []

USER nonroot

CMD ["uv", "run", "app/main.py"]
