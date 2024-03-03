# Build the manager binary
FROM golang:1.22 as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace/modes
COPY modes .

WORKDIR /workspace/op_freyr
COPY op_freyr/go.mod op_freyr/go.sum op_freyr/go.work op_freyr/go.work.sum ./
RUN go work use
RUN go mod download
RUN go mod verify
COPY op_freyr/ .

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/op_freyr/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
