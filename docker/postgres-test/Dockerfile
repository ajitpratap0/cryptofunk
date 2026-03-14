# PostgreSQL with TimescaleDB and pgvector for CI testing
# timescaledb-ha is Debian-based with TimescaleDB pre-installed.
# pgvector is installed via the PostgreSQL apt repository.

FROM timescale/timescaledb-ha:pg15-latest

USER root
RUN apt-get update \
    && apt-get install -y --no-install-recommends postgresql-15-pgvector \
    && rm -rf /var/lib/apt/lists/*
USER postgres
