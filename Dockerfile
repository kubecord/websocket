# Container for certs so we can use a scratch container at the end
FROM alpine:latest as certs
RUN apk --update add ca-certificates

# Build container
FROM golang:alpine as builder
WORKDIR /opt/build
RUN apk --no-cache add git
COPY . .
RUN CGO_ENABLED=0 go build -installsuffix 'static' -o /app

# Final product container
FROM scratch as final
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
