FROM golang:1.8-alpine

WORKDIR /go/src/github.com/matryer/vice

ADD . .

RUN apk add --update netcat-openbsd git && rm -rf /var/cache/apk/*

RUN go-wrapper download ./...

RUN go get github.com/matryer/is

RUN go-wrapper install ./...

ENTRYPOINT ["/bin/sh"]
