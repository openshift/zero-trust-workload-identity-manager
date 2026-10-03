# CI image for OIDC Discovery Provider — built from OpenShift midstream SPIRE extras.
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
  -o /usr/bin/oidc-discovery-provider ./support/oidc-discovery-provider

FROM registry.access.redhat.com/ubi9-minimal:latest
COPY --from=builder /usr/bin/oidc-discovery-provider /opt/spire/bin/oidc-discovery-provider
USER 65532:65532
ENTRYPOINT ["/opt/spire/bin/oidc-discovery-provider"]
