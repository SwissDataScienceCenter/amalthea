FROM python:3.8-slim

LABEL maintainer="info@datascience.ch"

RUN pip install --no-cache-dir --disable-pip-version-check -U pip && \
    pip install --no-cache-dir --disable-pip-version-check pipenv

# Install all packages
COPY Pipfile Pipfile.lock /app/
WORKDIR /app
RUN pipenv install --system --deploy

COPY controller /app/controller
ENV PYTHONPATH ".:${PYTHONPATH}"
ENTRYPOINT ["kopf", "run", "--liveness=http://0.0.0.0:8080/healthz", "/app/controller/server_controller.py"]
