ARG BUILDER_GOLANG_VERSION
# Build the manager binary
FROM --platform=linux/amd64 gcr.io/spectro-images-public/golang:${BUILDER_GOLANG_VERSION}-alpine as builder

ARG HELM=./bin/helm-linux-amd64
ARG HELM_CHART=./bin/vcluster-0.16.4.tgz
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Copy binaries
COPY ${HELM} helm
COPY ${HELM_CHART} vcluster-0.16.4.tgz

# Install Delve for debugging
RUN if [ "${TARGETARCH}" = "amd64" ]; then go install github.com/go-delve/delve/cmd/dlv@latest; fi


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
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 go build -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM --platform=linux/amd64 gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/helm .
COPY --from=builder /workspace/vcluster-0.16.4.tgz .
USER 65532:65532

ENTRYPOINT ["/manager"]
