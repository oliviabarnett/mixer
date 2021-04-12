FROM golang

ENV GOBIN=$GOPATH/bin

COPY . $GOPATH/src/github.com/oliviabarnett/mixer
WORKDIR $GOPATH/src/github.com/oliviabarnett/mixer

RUN go install github.com/oliviabarnett/mixer/cmd/mixer

ENTRYPOINT ["/go/bin/mixer"]