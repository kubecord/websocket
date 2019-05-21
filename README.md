# Kubecord Websock Handler
This is the websocket component of Kubecord.  It is responsible for capturing events from the Discord websocket,
forwarding them to NATS queues, and keeping the Redis cache consistent.  Borrowed heavily from [discordgo](https://github.com/bwmarrin/discordgo)
by [bwmarrin](https://github.com/bwmarrin).

## Building

### Binary
Checkout the code
```
git clone https://github.com/kubecord/websocket
```

Ensure you have go module support enabled
```
export GO111MODULE=on;
```

Build the binary
```
go build -o kubecord-ws
```

### Docker

Checkout the code
```
git clone https://github.com/kubecord/websocket
```

Build the container
```
docker build -t kubecord-ws .
```

## Binaries
Binaries will be provided once this project is at a point where we are confident that it meets
most of our desired functionality.  We will distribute them as releases on this repo,
and as Docker containers available on quay.io.

## Running

To run the websocket handler, you must set three required environment variables:

- `TOKEN` - Your bots token
- `REDIS_ADDR` - The address to your redis server or cluster
- `NATS_ADDR` - The address to your NATS server or cluster

### Examples

#### Binary
```sh
export TOKEN=mytokenhere
export REDIS_ADDR=localhost:6379
export NATS_ADDR=localhost
./kubecord-ws
```

#### Docker
```
docker run -e "TOKEN=mytokenhere" -e "REDIS_ADDR=localhost:6379" -e "NATS_ADDR=localhost" -d kubecord-ws
```

## Contributing

To contribute, please join our Discord server, you can find a link on the
 [meta repo](https://github.com/kubecord/Kubecord)