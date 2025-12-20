FROM golang:1.24.0-alpine AS builder

RUN apk add --no-cache make

WORKDIR /src
COPY . .
RUN make server OS=linux ARCH=amd64

FROM golang:1.24.0-alpine AS runner

WORKDIR /config
RUN cp /src/config/server_example.json ./server.json

WORKDIR /app
RUN mkdir certs logs

COPY --from=builder /src/build/gochatd .
COPY --from=builder /src/docker/docker-entrypoint.sh entrypoint.sh
RUN chmod +x entrypoint.sh

EXPOSE 9037/tcp
EXPOSE 8037/tcp

VOLUME ["/config"]

ENTRYPOINT ["/app/entrypoint.sh", "/app/gochatd"]
CMD ["--config", "/config/server.json"]