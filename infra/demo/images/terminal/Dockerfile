# The build context is produced by build_terminal_context.py, never a home tree.
FROM ubuntu:24.04@sha256:496754492fb28b4d3049432f2ca787449331e23fb14f0dd3fffea86bf5a93eb4
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3=3.12.3-0ubuntu2.1 python3-venv=3.12.3-0ubuntu2.1 ca-certificates=20260601~24.04.1 \
    && dpkg-query -W > /opt/feam-ubuntu-packages.txt && rm -rf /var/lib/apt/lists/*
COPY requirements.lock /opt/feam/requirements.lock
RUN python3 -m venv /opt/feam/venv && /opt/feam/venv/bin/pip install --no-cache-dir --require-hashes -r /opt/feam/requirements.lock
COPY feam /usr/local/bin/feam
COPY ttyd /usr/local/bin/ttyd
COPY model-adapter /usr/local/bin/model-adapter
COPY binary.sha256 /opt/feam/binary.sha256
RUN cd /usr/local/bin && sha256sum --check /opt/feam/binary.sha256 && chmod 0755 feam ttyd model-adapter
COPY feather-mesh/ /opt/feam/workspace/
ENV PATH=/opt/feam/venv/bin:/usr/local/bin:/usr/bin:/bin \
    FEAM_EXECUTABLE=/usr/local/bin/feam FEAM_WORKSPACE=/opt/feam/workspace \
    FEAM_CACHE_DIR=/workspace/cache XDG_STATE_HOME=/workspace/state \
    XDG_CONFIG_HOME=/workspace/config TERM=xterm-256color \
    PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
RUN pip install --no-deps --no-build-isolation /opt/feam/workspace/python_sdk \
    && python /opt/feam/workspace/mesh_core/tests/data/peer_access/generate_fixtures.py \
    && find /opt/feam/workspace/mesh_core/tests/data/peer_access -type f -exec sha256sum {} + > /opt/feam/fixtures.sha256 \
    && pip freeze > /opt/feam/python-packages.txt \
    && (getent passwd 1000 || useradd --uid 1000 --create-home --shell /bin/bash demo)
COPY entrypoint.sh launcher.sh hosted_check.py attach_releases.py /opt/feam/
COPY image_smoke.py /opt/feam/image_smoke.py
RUN chmod 0555 /opt/feam/entrypoint.sh /opt/feam/launcher.sh
USER 1000:1000
WORKDIR /workspace
ENTRYPOINT ["/opt/feam/entrypoint.sh"]
