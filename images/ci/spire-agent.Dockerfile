# CI image for SPIRE agent — built from OpenShift midstream (cert-manager pattern).
# Pin RELEASE_BRANCH to the ZTWIM 1.2 GA midstream branch that includes tls_config (SPIRE-714).
FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.26-openshift-4.22 AS builder

ARG RELEASE_BRANCH=release/v1.15.3
ARG GO_BUILD_TAGS=strictfipsruntime,openssl
ENV GOEXPERIMENT=strictfipsruntime
ENV CGO_ENABLED=1

RUN mkdir -p /go/src/github.com/spiffe
RUN git clone --depth 1 --branch "${RELEASE_BRANCH}" \
  https://github.com/openshift/spiffe-spire.git /go/src/github.com/spiffe/spire
WORKDIR /go/src/github.com/spiffe/spire

RUN go mod vendor
RUN go build -mod=vendor -tags "${GO_BUILD_TAGS}" -ldflags '-w -s' \
  -o /usr/bin/spire-agent ./cmd/spire-agent

FROM registry.access.redhat.com/ubi9-minimal:latest
COPY --from=builder /usr/bin/spire-agent /opt/spire/bin/spire-agent
# Agent runs as root by default so the Workload API socket can be shared in Kubernetes (upstream default).
USER 0:0
ENTRYPOINT ["/opt/spire/bin/spire-agent", "run"]
