# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.17-alpine as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} GO111MODULE=on go build -a -o manager main.go

# Pinned Alpine runtime: its CA bundle is the trust store the manager uses
FROM alpine:3.24@sha256:3f4d6b32e26703c6082839234f43769d5046b3ac7c910033911d1e71d8cd630a
WORKDIR /
COPY --from=builder /workspace/manager .
#USER nonroot:nonroot

ENTRYPOINT ["/manager"]
