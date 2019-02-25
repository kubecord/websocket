FROM golang:latest as builder
WORKDIR /go/src/gitea.auttaja.io/kubecord/ws
RUN go get github.com/gorilla/websocket \
    && go get github.com/go-redis/redis \
    && go get github.com/labstack/gommon/log \
    && go get github.com/nats-io/go-nats \
    && go get github.com/nats-io/go-nats-streaming
COPY . .
RUN CGO_ENABLED=0 go build -installsuffix 'static' -o /app .

FROM scratch as final
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
