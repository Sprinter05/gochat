FROM golang

WORKDIR /app

RUN git clone https://github.com/Sprinter05/gochat.git
RUN cd ./gochat
RUN make server
RUN mv ./build/gcserver /app/gochat
RUN mv ./config/example.json /app/config.json
RUN cd /app
RUN rm -r ./gochat
RUN mkdir logs

ENTRYPOINT ["./gochat"]