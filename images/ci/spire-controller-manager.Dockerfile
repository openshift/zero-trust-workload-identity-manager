# CI image for SPIRE Controller Manager — built from OpenShift midstream (cert-manager pattern).
# Pin RELEASE_BRANCH to the ZTWIM 1.2 GA midstream branch that includes tlsConfig (SPIRE-714).
FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.26-openshift-4.22 AS builder

ARG RELEASE_BRANCH=release/v0.7.0
ARG GO_BUILD_TAGS=strictfipsruntime,openssl
ENV GOEXPERIMENT=strictfipsruntime
ENV CGO_ENABLED=1

RUN mkdir -p /go/src/github.com/spiffe
RUN git clone --depth 1 --branch "${RELEASE_BRANCH}" \
  https://github.com/openshift/spiffe-spire-controller-manager.git \
  /go/src/github.com/spiffe/spire-controller-manager
WORKDIR /go/src/github.com/spiffe/spire-controller-manager

RUN go mod vendor
RUN go build -mod=vendor -tags "${GO_BUILD_TAGS}" -ldflags '-w -s' \
  -o /usr/bin/spire-controller-manager ./cmd/main.go

FROM registry.access.redhat.com/ubi9-minimal:latest
COPY --from=builder /usr/bin/spire-controller-manager /usr/bin/spire-controller-manager
USER 65532:65532
ENTRYPOINT ["/usr/bin/spire-controller-manager"]
