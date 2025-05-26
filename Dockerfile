# Build the Zero Trust Workload Identity Manager binary
FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.23-openshift-4.18 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

COPY . .

RUN go mod download

# Build
RUN CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -mod=mod -a -o zero-trust-workload-identity-manager ./cmd/zero-trust-workload-identity-manager/main.go

FROM registry.access.redhat.com/ubi9-minimal:9.4
WORKDIR /
COPY --from=builder /workspace/zero-trust-workload-identity-manager .
USER 65532:65532

ENTRYPOINT ["/zero-trust-workload-identity-manager"]
