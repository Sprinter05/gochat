FROM golang

WORKDIR /app

RUN git clone https://github.com/Sprinter05/gochat.git

WORKDIR /app/gochat

RUN make server
RUN mv ./build/gcserver /app
RUN mv ./config/example.json /app/config.json

WORKDIR /app

RUN rm -r ./gochat
RUN mkdir logs
RUN mv ./gcserver ./gochat
RUN chmod +x ./gochat

ENTRYPOINT ["/app/gochat"]