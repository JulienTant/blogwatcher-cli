FROM alpine:3.21 AS certs
RUN apk add --no-cache ca-certificates

# --- Pre-built binary (goreleaser: --target=release) ---
FROM scratch AS release
ARG TARGETPLATFORM
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY ${TARGETPLATFORM}/blogwatcher-cli /blogwatcher-cli
VOLUME /data
ENV BLOGWATCHER_DB=/data/blogwatcher-cli.db
USER 65532:65532
ENTRYPOINT ["/blogwatcher-cli"]

# --- Build from source (default: docker build .) ---
FROM alpine:3.21 AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src
COPY go.mod go.sum ./

# Install the exact Go version from go.mod, with checksum verification
RUN GO_VERSION=$(grep '^go ' go.mod | awk '{print $2}') && \
    TARBALL="go${GO_VERSION}.${TARGETOS}-${TARGETARCH}.tar.gz" && \
    wget -q "https://dl.google.com/go/${TARBALL}" -O /tmp/go.tar.gz && \
    EXPECTED=$(wget -qO- "https://dl.google.com/go/${TARBALL}.sha256") && \
    echo "${EXPECTED}  /tmp/go.tar.gz" | sha256sum -c - && \
    tar -C /usr/local -xzf /tmp/go.tar.gz && \
    rm /tmp/go.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"

RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /blogwatcher-cli ./cmd/blogwatcher-cli

FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /blogwatcher-cli /blogwatcher-cli
VOLUME /data
ENV BLOGWATCHER_DB=/data/blogwatcher-cli.db
USER 65532:65532
ENTRYPOINT ["/blogwatcher-cli"]
