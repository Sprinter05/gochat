# This allows for a different version to be used
ARG GOVERSION=latest
FROM golang:$GOVERSION

# Copy the source code and compile
WORKDIR /src

COPY ../server .
COPY ../Makefile .
RUN make server

# Create configuration
WORKDIR /config

VOLUME ["/config"]
COPY ../config/server_example.json /config/server.json

# Copy the app binary and create necessary folders
WORKDIR /app

RUN cp /src/build/gochat-server .
RUN mkdir certs logs

# Forward ports
EXPOSE 9037/tcp
EXPOSE 8037/tcp

# Set binary and parameters
ENTRYPOINT ["/app/gochat-server"]
CMD ["--config", "/config/server.json"]