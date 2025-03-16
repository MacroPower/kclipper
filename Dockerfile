FROM debian:12-slim

ENV LANG=en_US.utf8 \
    XDG_CACHE_HOME=/tmp/xdg_cache \
    KCL_LIB_HOME=/tmp/kcl_lib \
    KCL_PKG_PATH=/tmp/kcl_pkg \
    KCL_CACHE_PATH=/tmp/kcl_cache \
    KCL_FAST_EVAL=1

ARG TARGETOS
ARG TARGETARCH

RUN apt-get update && \
    apt-get install curl gpg apt-transport-https --yes && \
    rm -rf /var/lib/apt/lists/* && rm -rf /tmp/*

COPY kcl /usr/local/bin/

RUN kcl version && \
    echo 'a=1' | kcl run -

ENTRYPOINT ["kcl"]
