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

FROM rockylinux/rockylinux:10

# Install OLS core without bundled lsphp (LLStack uses Remi php-litespeed instead per ADR-0012).
# Uses EL9 repo as OLS has no dedicated EL10 packages yet.
RUN dnf -y install dnf sudo findutils tar gzip procps-ng iproute hostname shadow-utils curl \
  && printf '[litespeed]\nname=LiteSpeed Tech\nbaseurl=http://rpms.litespeedtech.com/centos/9/$basearch/\nenabled=1\ngpgcheck=0\n' > /etc/yum.repos.d/litespeed.repo \
  && (dnf download openlitespeed --destdir=/tmp/ols-rpms 2>/dev/null && rpm -ivh --nodeps /tmp/ols-rpms/openlitespeed-*.rpm 2>/dev/null && rm -rf /tmp/ols-rpms \
      || echo "OLS core install unavailable on this architecture, using asset-only mode") \
  && dnf clean all

COPY --from=build /out/llstack /usr/local/bin/llstack
COPY docker/fixtures/smoke.sh /usr/local/bin/llstack-docker-smoke
COPY docker/fixtures/systemctl-ols.sh /usr/local/bin/systemctl

RUN chmod +x /usr/local/bin/llstack /usr/local/bin/llstack-docker-smoke /usr/local/bin/systemctl \
  && ([ -f /usr/local/lsws/bin/lswsctrl ] && ln -sf /usr/local/lsws/bin/lswsctrl /usr/local/bin/lswsctrl || true) \
  && mkdir -p /usr/local/lsws/conf/vhosts /usr/local/lsws/conf/llstack/listeners /var/lib/llstack/state/parity /data/www \
  && ([ -f /usr/local/lsws/conf/httpd_config.conf ] && sed -i 's/address.*\*:8088/address                 *:80/' /usr/local/lsws/conf/httpd_config.conf && sed -i '/map.*Example/d' /usr/local/lsws/conf/httpd_config.conf || true)

ENV LLSTACK_SMOKE_BACKEND=ols

CMD ["/usr/local/bin/llstack-docker-smoke"]
