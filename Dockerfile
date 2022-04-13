FROM python:3.8-slim

LABEL maintainer="info@datascience.ch"

RUN pip install --no-cache-dir --disable-pip-version-check -U pip && \
    pip install --no-cache-dir --disable-pip-version-check pipenv && \
    groupadd -g 1000 amalthea && \
    useradd -u 1000 -g 1000 amalthea

# Install all packages
WORKDIR /app
COPY Pipfile Pipfile.lock ./
RUN pipenv install --system --deploy

COPY controller /app/controller
COPY kopf_entrypoint.py ./

USER 1000:1000
ENTRYPOINT ["kopf", "run", "--liveness=http://0.0.0.0:8080/healthz", "./kopf_entrypoint.py"]
