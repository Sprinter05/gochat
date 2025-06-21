#!/bin/sh

if [ ! -f "/config/server.json" ] then
    cp /src/config/server_example.json /config/server.json
fi

exec "$@"