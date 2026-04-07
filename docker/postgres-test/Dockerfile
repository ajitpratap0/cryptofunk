# PostgreSQL with TimescaleDB and pgvector for CI testing
# Uses the standard TimescaleDB Alpine image (not -ha, which requires Patroni).
# pgvector is built from source since Alpine doesn't have a prebuilt package.
#
# We build with `with_llvm=no` to skip the LLVM bitcode (JIT) step. The base
# image's pg_config reports whichever clang version was used to build the
# upstream postgres package, and that drifts over time (clang15 -> clang19 -> ...).
# Pinning a clang version here breaks every time the base image rolls forward.
# pgvector still works fine without JIT bitcode — the .so is the only thing we
# actually need for tests.

FROM timescale/timescaledb:latest-pg15

USER root
RUN apk add --no-cache --virtual .build-deps \
        build-base \
        git \
    && cd /tmp \
    && git clone --branch v0.8.0 https://github.com/pgvector/pgvector.git \
    && cd pgvector \
    && make OPTFLAGS="" with_llvm=no \
    && make OPTFLAGS="" with_llvm=no install \
    && cd / \
    && rm -rf /tmp/pgvector \
    && apk del .build-deps
USER postgres
