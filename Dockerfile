FROM python:3.8-slim

LABEL maintainer="info@datascience.ch"

RUN pip install --no-cache-dir --disable-pip-version-check -U pip && \
    pip install --no-cache-dir --disable-pip-version-check pipenv && \
    apt-get update && \
    apt-get install tini -y && \
    rm -rf /var/lib/apt/lists/*

# Install all packages
WORKDIR /app
COPY Pipfile Pipfile.lock ./
RUN pipenv install --system --deploy

COPY controller /app/controller
COPY kopf_entrypoint.py ./

ENTRYPOINT ["tini", "-g", "--", "kopf", "run", "--liveness=http://0.0.0.0:8080/healthz", "./kopf_entrypoint.py"]
