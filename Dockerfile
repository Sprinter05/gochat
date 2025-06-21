# This allows for a different version to be used
ARG GOVERSION=latest
FROM golang:$GOVERSION

# Copy the source code and compile
WORKDIR /src
COPY . .
RUN make server

# Copy the app binary and create config folders
WORKDIR /app
VOLUME ["/config"]
RUN mv /src/config/server_example.json /config/server.json &&\
    cp /src/build/gochat-server . &&\
    mkdir certs logs

# Forward ports
EXPOSE 9037/tcp
EXPOSE 8037/tcp

# Set binary and parameters
ENTRYPOINT ["/app/gochat-server"]
CMD ["--config", "/config/server.json"]