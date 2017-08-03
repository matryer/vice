FROM golang:1.8-alpine

WORKDIR /go/src/github.com/matryer/vice

ADD . .

RUN go-wrapper download

RUN go-wrapper install

ENTRYPOINT ["/bin/sh"]
