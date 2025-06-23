# Using alpine for size optimisation
FROM golang:alpine

# Install necessary packages
RUN apk add --no-cache make

# Copy the source code and compile
WORKDIR /src
COPY . .
RUN make server

# Setup configuration files
WORKDIR /config
RUN cp /src/config/server_example.json ./server.json

# Copy the app binary and entrypoint, then create necessary folders
WORKDIR /app
RUN mkdir certs logs &&\
    cp /src/build/gochat-server . &&\
    cp /src/docker/docker-entrypoint.sh entrypoint.sh &&\
    chmod +x entrypoint.sh

# Forward ports
EXPOSE 9037/tcp
EXPOSE 8037/tcp

# Set volumes
VOLUME ["/config"]

# Set binary and parameters
ENTRYPOINT ["/app/entrypoint.sh", "/app/gochat-server"]
CMD ["--config", "/config/server.json"]