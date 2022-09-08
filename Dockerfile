FROM python:3.9-slim

LABEL maintainer="info@datascience.ch"

RUN pip install --no-cache-dir --disable-pip-version-check -U pip && \
    pip install --no-cache-dir --disable-pip-version-check pipenv && \
    groupadd -g 1000 amalthea && \
    useradd -u 1000 -g 1000 amalthea && \
    apt-get update && \
    apt-get install tini curl -y && \
    rm -rf /var/lib/apt/lists/*

# Install all packages
WORKDIR /app
COPY Pipfile Pipfile.lock ./
RUN pipenv install --system --deploy
RUN curl -L https://github.com/NVIDIA/container-canary/releases/download/v0.2.1/canary_linux_amd64 > canary_linux_amd64 && \
    curl -L https://github.com/NVIDIA/container-canary/releases/download/v0.2.1/canary_linux_amd64.sha256sum > canary_linux_amd64.sha256sum && \
    sha256sum --check --status canary_linux_amd64.sha256sum && \
    chmod +x canary_linux_amd64 && \
    mv canary_linux_amd64 /usr/local/bin/canary
# fix pyngrok permissions
RUN mkdir /usr/local/lib/python3.9/site-packages/pyngrok/bin && \
    chown :1000 /usr/local/lib/python3.9/site-packages/pyngrok/bin && \
    chmod 770 /usr/local/lib/python3.9/site-packages/pyngrok/bin && \
    mkdir /home/amalthea/ && \
    chown :1000 /home/amalthea && \
    chmod 770 /home/amalthea

COPY controller /app/controller
COPY kopf_entrypoint.py ./

USER 1000:1000
ENTRYPOINT ["tini", "-g", "--", "kopf", "run", "--liveness=http://0.0.0.0:8080/healthz", "./kopf_entrypoint.py"]
