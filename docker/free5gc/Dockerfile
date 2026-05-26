FROM ubuntu:22.04 AS builder

ARG NF
ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      git \
      make \
      xz-utils \
    && rm -rf /var/lib/apt/lists/*

ARG GO_VERSION=1.25.5
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
      | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /src/free5gc
COPY . .

RUN test -n "${NF}" \
    && test -d "NFs/${NF}/cmd" \
    && make "${NF}"

FROM ubuntu:22.04

ARG NF
ARG DEBIAN_FRONTEND=noninteractive
ENV NF=${NF}

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
      ca-certificates \
      iproute2 \
      iptables \
      netbase \
      tini \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /free5gc

COPY --from=builder /src/free5gc/bin/${NF} /free5gc/${NF}
COPY cert /free5gc/cert

RUN chmod +x "/free5gc/${NF}" \
    && ln -sf /usr/bin/tini /sbin/tini

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["sh", "-c", "exec /free5gc/${NF}"]
