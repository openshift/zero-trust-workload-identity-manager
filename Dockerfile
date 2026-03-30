# Build the Zero Trust Workload Identity Manager binary
FROM registry.redhat.io/ubi10/go-toolset:10.1 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
USER root

COPY . .

RUN go mod download

# Build
RUN CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -mod=mod -a -o zero-trust-workload-identity-manager ./cmd/zero-trust-workload-identity-manager/main.go

FROM registry.redhat.io/ubi10:10.1
WORKDIR /
COPY --from=builder /workspace/zero-trust-workload-identity-manager /usr/bin
USER 65532:65532

ENTRYPOINT ["/usr/bin/zero-trust-workload-identity-manager"]
