FROM golang:alpine as builder

RUN apk add git
RUN go get -v -u github.com/golang/dep/cmd/dep

ENV PACKAGE=github.com/openchirp/openchirp_pubsub
ENV BINARY=openchirp_pubsub

RUN mkdir -p /go/src/$PACKAGE
COPY . /go/src/$PACKAGE
WORKDIR /go/src/$PACKAGE
RUN dep ensure
RUN go install

FROM alpine:latest

WORKDIR /root
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /go/bin/$BINARY .
ENTRYPOINT ["./openchirp_pubsub"]