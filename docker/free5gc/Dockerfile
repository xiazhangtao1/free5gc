FROM ubuntu:22.04 AS builder

ARG NF
ARG DEBIAN_FRONTEND=noninteractive
ARG OFFLINE_BUILD=auto
ARG GO_VERSION=1.25.5
ARG GOPROXY=https://proxy.golang.org,direct
ARG GOSUMDB=sum.golang.org

COPY deploy/offline/cache /offline-cache

RUN if [ "${OFFLINE_BUILD}" != "false" ] && [ -f /offline-cache/apt/builder/Packages.gz ]; then \
      printf 'deb [trusted=yes] file:/offline-cache/apt/builder ./\n' > /etc/apt/sources.list; \
    elif [ "${OFFLINE_BUILD}" = "true" ]; then \
      echo "offline apt cache is missing: deploy/offline/cache/apt/builder" >&2; \
      exit 1; \
    fi \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      git \
      make \
      xz-utils \
    && rm -rf /var/lib/apt/lists/*

RUN if [ "${OFFLINE_BUILD}" != "false" ] && [ -f "/offline-cache/go/go${GO_VERSION}.linux-amd64.tar.gz" ]; then \
      tar -C /usr/local -xzf "/offline-cache/go/go${GO_VERSION}.linux-amd64.tar.gz"; \
    elif [ "${OFFLINE_BUILD}" = "true" ]; then \
      echo "offline Go tarball is missing: deploy/offline/cache/go/go${GO_VERSION}.linux-amd64.tar.gz" >&2; \
      exit 1; \
    else \
      curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
        | tar -C /usr/local -xz; \
    fi
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH=/go

WORKDIR /src/free5gc
COPY . .

RUN test -n "${NF}" \
    && test -d "NFs/${NF}/cmd" \
    && if [ "${OFFLINE_BUILD}" != "false" ] && [ -d /offline-cache/gomod/pkg/mod/cache/download ]; then \
         mkdir -p /go/pkg/mod; \
         cp -a /offline-cache/gomod/pkg/mod/. /go/pkg/mod/; \
         GOPROXY=off GOSUMDB=off make "${NF}"; \
       elif [ "${OFFLINE_BUILD}" = "true" ]; then \
         echo "offline Go module cache is missing: deploy/offline/cache/gomod/pkg/mod/cache/download" >&2; \
         exit 1; \
       else \
         GOPROXY="${GOPROXY}" GOSUMDB="${GOSUMDB}" make "${NF}"; \
       fi

FROM ubuntu:22.04

ARG NF
ARG DEBIAN_FRONTEND=noninteractive
ARG OFFLINE_BUILD=auto
ENV NF=${NF}

COPY deploy/offline/cache /offline-cache

RUN if [ "${OFFLINE_BUILD}" != "false" ] && [ -f /offline-cache/apt/runtime/Packages.gz ]; then \
      printf 'deb [trusted=yes] file:/offline-cache/apt/runtime ./\n' > /etc/apt/sources.list; \
    elif [ "${OFFLINE_BUILD}" = "true" ]; then \
      echo "offline apt cache is missing: deploy/offline/cache/apt/runtime" >&2; \
      exit 1; \
    fi \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
      ca-certificates \
      iproute2 \
      iptables \
      iperf \
      iperf3 \
      netbase \
      tcpdump \
      tini \
    && rm -rf /var/lib/apt/lists/* /offline-cache

WORKDIR /free5gc

COPY --from=builder /src/free5gc/bin/${NF} /free5gc/${NF}
COPY cert /free5gc/cert

RUN chmod +x "/free5gc/${NF}" \
    && ln -sf /usr/bin/tini /sbin/tini

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["sh", "-c", "exec /free5gc/${NF}"]
