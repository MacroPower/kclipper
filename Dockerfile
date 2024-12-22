FROM --platform=${BUILDPLATFORM} debian:11
ENV LANG=en_US.utf8

ARG HELM_VERSION=3.16.4
ARG TARGETOS
ARG TARGETARCH

RUN apt-get update && \
    apt-get install curl gpg apt-transport-https --yes && \
    rm -rf /var/lib/apt/lists/*

RUN curl -Lq https://get.helm.sh/helm-v${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz | \
    tar -xzO ${TARGETOS}-${TARGETARCH}/helm > /usr/local/bin/helm && \
    chmod +x /usr/local/bin/helm

# RUN helm version

COPY kcl /usr/local/bin/

ENV KCL_LIB_HOME=/tmp \
    KCL_PKG_PATH=/tmp \
    KCL_CACHE_PATH=/tmp \
    KCL_FAST_EVAL=1

# RUN kcl version && \
#     echo 'a=1' | kcl run -

ENTRYPOINT ["kcl"]
