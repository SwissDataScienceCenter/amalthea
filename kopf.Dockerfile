FROM python:3.12-bookworm as builder
ARG DEV_BUILD=false
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN groupadd --gid $USER_GID renku && \
    DEBIAN_FRONTEND=noninteractive adduser --gid $USER_GID --uid $USER_UID renku && \
    apt-get update && apt-get install -y tini
USER $USER_UID:$USER_GID
WORKDIR /app
RUN python3 -m pip install --user pipx && \
    python3 -m pipx ensurepath && \
    /home/renku/.local/bin/pipx install poetry && \
    /home/renku/.local/bin/pipx install virtualenv && \
    /home/renku/.local/bin/virtualenv env
COPY --chown=$USER_UID:$USER_GID . .
RUN if $DEV_BUILD ; then \
    /home/renku/.local/bin/poetry export -o requirements.txt --with dev; \
    env/bin/pip install -r requirements.txt; \
  fi
RUN /home/renku/.local/bin/poetry build -f wheel 
RUN env/bin/pip --no-cache-dir install dist/*.whl

FROM python:3.12-slim-bookworm
ARG USER_UID=1000
ARG USER_GID=$USER_UID
ENV PROMETHEUS_MULTIPROC_DIR=/prometheus
RUN mkdir /prometheus && \
    groupadd --gid $USER_GID renku && \
    adduser --gid $USER_GID --uid $USER_UID renku
USER $USER_UID:$USER_GID
WORKDIR /app
COPY --from=builder /usr/bin/tini /usr/bin/tini
COPY --from=builder /app/env ./env
ENTRYPOINT ["tini", "-g", "--", "./env/bin/python", "-m", "controller.main"]
