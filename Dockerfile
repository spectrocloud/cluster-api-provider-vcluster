ARG BUILDER_GOLANG_VERSION
ARG BUILDER_3RDPARTY_VERSION
# Build the manager binary
FROM --platform=$TARGETPLATFORM gcr.io/spectro-images-public/builders/spectro-third-party:${BUILDER_3RDPARTY_VERSION} as thirdparty
FROM --platform=linux/amd64 gcr.io/spectro-images-public/golang:${BUILDER_GOLANG_VERSION}-alpine as builder

ENV BIN_TYPE=${CRYPTO_LIB:+vertex}
ENV BIN_TYPE=${BIN_TYPE:-palette}

ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Copy binaries
COPY ${HELM} helm
COPY --from=thirdparty /binaries/helm/latest/$BIN_TYPE/$TARGETARCH/helm /binaries/helm
COPY --from=thirdparty /binaries/helm/latest/$BIN_TYPE/$TARGETARCH/helm helm
COPY ${HELM_CHART} vcluster-0.18.1.tgz

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
COPY --from=builder /workspace/vcluster-0.18.1.tgz .
USER 65532:65532

ENTRYPOINT ["/manager"]
