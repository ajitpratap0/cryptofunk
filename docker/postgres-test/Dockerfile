# PostgreSQL with TimescaleDB and pgvector for CI testing
# Based on TimescaleDB image (Alpine), adds pgvector extension

FROM timescale/timescaledb:latest-pg15

# Install build dependencies for pgvector (Alpine uses apk, not apt-get)
USER root
RUN apk add --no-cache --virtual .build-deps \
    build-base \
    git \
    clang15 \
    llvm15-dev \
    && apk add --no-cache postgresql15-dev

# Clone and install pgvector
RUN cd /tmp \
    && git clone --branch v0.8.0 https://github.com/pgvector/pgvector.git \
    && cd pgvector \
    && make \
    && make install \
    && cd / \
    && rm -rf /tmp/pgvector

# Clean up build dependencies to reduce image size
RUN apk del .build-deps

USER postgres
