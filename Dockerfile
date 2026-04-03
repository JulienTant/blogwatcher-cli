FROM alpine:3.21 AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src
COPY go.mod go.sum ./

# Install the exact Go version from go.mod
RUN GO_VERSION=$(grep '^go ' go.mod | awk '{print $2}') && \
    wget -q "https://go.dev/dl/go${GO_VERSION}.${TARGETOS}-${TARGETARCH}.tar.gz" -O /tmp/go.tar.gz && \
    tar -C /usr/local -xzf /tmp/go.tar.gz && \
    rm /tmp/go.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"

RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /blogwatcher-cli ./cmd/blogwatcher-cli

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /blogwatcher-cli /blogwatcher-cli
ENTRYPOINT ["/blogwatcher-cli"]
