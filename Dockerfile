ARG BUILDER_GOLANG_VERSION
ARG BUILDER_3RDPARTY_VERSION
# Build the manager binary
FROM --platform=$TARGETPLATFORM us-central1-docker.pkg.dev/palette-images-dev/hardened-images/third-party/spectro-third-party:${BUILDER_3RDPARTY_VERSION} AS thirdparty
FROM --platform=linux/amd64 us-central1-docker.pkg.dev/palette-images-dev/hardened-images/builder/golang:${BUILDER_GOLANG_VERSION}-alpine AS builder


ARG TARGETOS
ARG TARGETARCH
ARG CRYPTO_LIB

# CRYPTO_LIB set -> vertex (FIPS) third-party binaries; unset -> palette
ENV BIN_TYPE=${CRYPTO_LIB:+vertex}
ENV BIN_TYPE=${BIN_TYPE:-palette}

WORKDIR /workspace

# Copy binaries
COPY --from=thirdparty /binaries/helm/latest/$BIN_TYPE/$TARGETARCH/helm helm

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

# Copy vCluster charts
COPY charts/ /charts/

# CRYPTO_LIB set -> FIPS 140-3 build with the Go Cryptographic Module v1.0.0
# (GOFIPS140, NIST CMVP cert #5247). Pure Go: no cgo, no external linking, and
# no fipsonly shim - those were boringcrypto (FIPS 140-2) requirements.
RUN if [ "${CRYPTO_LIB}" ]; then \
      CGO_ENABLED=0 GOFIPS140=v1.0.0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
      go build -a -o manager main.go; \
    else \
      go-build-static.sh -a -o manager main.go; \
    fi

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM --platform=linux/amd64 gcr.io/distroless/static:nonroot

ARG CRYPTO_LIB
# FIPS builds: enforce FIPS 140-3 mode at runtime (Go native module).
# Override to GODEBUG=fips140=off to run a FIPS build in non-FIPS mode.
ENV GODEBUG=${CRYPTO_LIB:+fips140=on}

WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/helm .
COPY --from=builder /charts/ /charts/
USER 65532:65532

ENTRYPOINT ["/manager"]
