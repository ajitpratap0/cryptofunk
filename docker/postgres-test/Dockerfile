# PostgreSQL with TimescaleDB and pgvector for CI testing
# Based on TimescaleDB image, adds pgvector extension

FROM timescale/timescaledb:latest-pg15

# Install build dependencies for pgvector
USER root
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    git \
    postgresql-server-dev-15 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Clone and install pgvector
RUN cd /tmp \
    && git clone --branch v0.8.0 https://github.com/pgvector/pgvector.git \
    && cd pgvector \
    && make \
    && make install \
    && cd / \
    && rm -rf /tmp/pgvector

# Clean up build dependencies to reduce image size
RUN apt-get purge -y --auto-remove build-essential git postgresql-server-dev-15 \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

USER postgres
