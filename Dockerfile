# Build the Zero Trust Workload Identity Manager binary
FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.26-openshift-5.0 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

COPY . .

RUN go mod download

# Build
RUN CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -mod=mod -a -o zero-trust-workload-identity-manager ./cmd/zero-trust-workload-identity-manager/main.go

FROM registry.redhat.io/ubi9/ubi-minimal-pqc:latest
WORKDIR /
COPY --from=builder /workspace/zero-trust-workload-identity-manager /usr/bin
USER 65532:65532

ENTRYPOINT ["/usr/bin/zero-trust-workload-identity-manager"]
