# Build the manager binary
FROM golang:1.25-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
ARG RESTIC_VERSION=0.18.1

# Install ca-certificates for HTTPS and git for go mod
RUN apk add --no-cache ca-certificates git wget bzip2

# Download restic binary
RUN wget -O /restic.bz2 "https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_${TARGETARCH:-amd64}.bz2" \
    && bunzip2 /restic.bz2 \
    && chmod +x /restic

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/

# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -a -ldflags="-w -s" -o manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /restic /usr/local/bin/restic
USER 65532:65532

ENTRYPOINT ["/manager"]
