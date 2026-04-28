FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY main.py filters.yaml suppress.yaml context.md ./

RUN useradd --system --no-create-home --uid 10001 reviewer
USER 10001

ENTRYPOINT ["python", "/app/main.py"]
