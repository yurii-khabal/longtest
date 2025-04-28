FROM golang:1.18-alpine3.17 as builder

COPY . /longtest

WORKDIR /longtest

RUN go build -o longtest .

FROM alpine:3.19

COPY --from=builder /longtest/longtest /longtest

CMD /longtest