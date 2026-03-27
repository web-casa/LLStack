FROM golang:1.24 AS build

WORKDIR /src
COPY . .

ARG LLSTACK_VERSION=0.1.0-docker
ARG LLSTACK_COMMIT=unknown
ARG LLSTACK_BUILD_DATE=unknown
ARG TARGETOS
ARG TARGETARCH

RUN target_os="${TARGETOS:-$(go env GOOS)}" && target_arch="${TARGETARCH:-$(go env GOARCH)}" && \
  CGO_ENABLED=0 GOOS="${target_os}" GOARCH="${target_arch}" go build \
  -trimpath \
  -ldflags "-s -w -X main.version=${LLSTACK_VERSION} -X main.commit=${LLSTACK_COMMIT} -X main.buildDate=${LLSTACK_BUILD_DATE} -X main.targetOS=${target_os} -X main.targetArch=${target_arch}" \
  -o /out/llstack \
  ./cmd/llstack

FROM rockylinux:9

RUN dnf -y install dnf sudo findutils tar gzip procps-ng iproute hostname shadow-utils && dnf clean all

COPY --from=build /out/llstack /usr/local/bin/llstack
COPY docker/fixtures/smoke.sh /usr/local/bin/llstack-docker-smoke

RUN chmod +x /usr/local/bin/llstack /usr/local/bin/llstack-docker-smoke \
  && mkdir -p /usr/local/lsws/conf/llstack/includes /var/lib/llstack/state/parity /usr/local/lsws/conf \
  && printf '%s\n' 'trial-license' >/usr/local/lsws/conf/serial.no

ENV LLSTACK_SMOKE_BACKEND=lsws

CMD ["/usr/local/bin/llstack-docker-smoke"]
