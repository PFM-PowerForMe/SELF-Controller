FROM docker.io/library/golang:alpine AS builder
ARG REPO
ARG ARCH
ARG CPU_ARCH
ARG TAG=dev
ARG IMAGE_VERSION
ENV REPO=$REPO \
     ARCH=$ARCH \
     CPU_ARCH=$CPU_ARCH \
     TAG=$TAG \
     IMAGE_VERSION=$IMAGE_VERSION

ENV CGO_ENABLED=0 \
     GOOS=linux \
     GOARCH=$ARCH

WORKDIR /output/
WORKDIR /source/
COPY source-src/src/ ./
RUN go mod download
RUN go build -o /output/controller -trimpath -ldflags="-w -s -X main.version=${TAG}" .

FROM scratch AS runtime
COPY --from=builder /output/controller /usr/bin/controller
ENTRYPOINT ["/usr/bin/controller"]
