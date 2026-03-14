# PostgreSQL with TimescaleDB and pgvector for CI testing
# Uses the standard TimescaleDB Alpine image (not -ha, which requires Patroni).
# pgvector is built from source since Alpine doesn't have a prebuilt package.

FROM timescale/timescaledb:latest-pg15

USER root
RUN apk add --no-cache --virtual .build-deps \
        build-base \
        git \
        clang15 \
        llvm15-dev \
    && cd /tmp \
    && git clone --branch v0.8.0 https://github.com/pgvector/pgvector.git \
    && cd pgvector \
    && make \
    && make install \
    && cd / \
    && rm -rf /tmp/pgvector \
    && apk del .build-deps
USER postgres
