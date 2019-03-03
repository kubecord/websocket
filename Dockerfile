# Container for certs so we can use a scratch container at the end
FROM alpine:latest as certs
RUN apk --update add ca-certificates

# Build container
FROM golang:latest as builder
WORKDIR /go/src/gitea.auttaja.io/kubecord/ws
RUN go get github.com/gorilla/websocket \
    && go get github.com/go-redis/redis \
    && go get github.com/labstack/gommon/log \
    && go get github.com/nats-io/go-nats \
    && go get github.com/nats-io/go-nats-streaming \
    && go get github.com/imdario/mergo
COPY . .
RUN CGO_ENABLED=0 go build -installsuffix 'static' -o /app .

# Final product container
FROM scratch as final
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
