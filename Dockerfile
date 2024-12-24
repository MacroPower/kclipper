FROM debian:12-slim

ENV LANG=en_US.utf8 \
    KCL_LIB_HOME=/tmp \
    KCL_PKG_PATH=/tmp \
    KCL_CACHE_PATH=/tmp \
    KCL_FAST_EVAL=1

ARG HELM_VERSION
ARG TARGETOS
ARG TARGETARCH

RUN apt-get update && \
    apt-get install curl gpg apt-transport-https --yes && \
    rm -rf /var/lib/apt/lists/* && rm -rf /tmp/*

RUN curl -Lq https://get.helm.sh/helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz | \
    tar -xzO ${TARGETOS}-${TARGETARCH}/helm > /usr/local/bin/helm && \
    chmod +x /usr/local/bin/helm && \
    helm version

COPY kcl /usr/local/bin/

RUN kcl version && \
    echo 'a=1' | kcl run -

ENTRYPOINT ["kcl"]
