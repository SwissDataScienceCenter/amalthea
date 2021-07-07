FROM python:3.8-slim

LABEL maintainer="info@datascience.ch"

RUN pip install --no-cache-dir --disable-pip-version-check -U pip && \
    pip install --no-cache-dir --disable-pip-version-check pipenv

# Install all packages
COPY Pipfile Pipfile.lock /app/
WORKDIR /app
RUN pipenv install --system --deploy

COPY controller/src /app/src
ENTRYPOINT ["kopf", "run", "--liveness=http://0.0.0.0:8080/healthz", "/app/src/server_controller.py"]
