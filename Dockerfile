# This allows for a different version to be used
ARG GOVERSION=latest
FROM golang:$GOVERSION

# Copy the source code and compile
WORKDIR /src
COPY . .
RUN make server

# Setup configuration files
WORKDIR /config
RUN mv /src/config/server_example.json ./server.json

# Copy the app binary and create necessary folders
WORKDIR /app
RUN mkdir certs logs &&\
    cp /src/build/gochat-server .

# Forward ports
EXPOSE 9037/tcp
EXPOSE 8037/tcp

# Set volumes
VOLUME ["/config"]

# Set binary and parameters
ENTRYPOINT ["/app/gochat-server"]
CMD ["--config", "/config/server.json"]