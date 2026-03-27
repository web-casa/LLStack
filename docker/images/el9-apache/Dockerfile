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

RUN dnf -y install dnf sudo findutils tar gzip procps-ng iproute hostname shadow-utils httpd curl-minimal && dnf clean all

COPY --from=build /out/llstack /usr/local/bin/llstack
COPY docker/fixtures/smoke.sh /usr/local/bin/llstack-docker-smoke
COPY docker/fixtures/systemctl-httpd.sh /usr/local/bin/systemctl

RUN chmod +x /usr/local/bin/llstack /usr/local/bin/llstack-docker-smoke /usr/local/bin/systemctl \
  && mkdir -p /etc/httpd/conf.d/llstack/sites /run/httpd /data/www \
  && chmod 755 /data /data/www \
  && printf '%s\n' 'IncludeOptional /etc/httpd/conf.d/llstack/sites/*.conf' >/etc/httpd/conf.d/00-llstack-managed-sites.conf

CMD ["/usr/local/bin/llstack-docker-smoke"]
