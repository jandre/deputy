FROM golang:1.5-alpine

ADD vendor/ /go/src
ADD src /go/src
RUN go install bot 

CMD ["/go/bin/bot"]
