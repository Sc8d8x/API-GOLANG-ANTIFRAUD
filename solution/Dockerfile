FROM golang:1.25-alpine3.21 AS builder

RUN apk add --no-cache ca-certificates git gcc g++ libc-dev binutils

WORKDIR /opt


COPY solution/go.mod solution/go.sum ./
RUN go mod download && go mod verify


COPY ./solution .


RUN go build -o bin/application ./cmd

FROM alpine:3.21 AS runner

RUN apk add --no-cache ca-certificates libc6-compat openssh bash curl

WORKDIR /opt

COPY --from=builder /opt/bin/application ./


CMD ["./application"]
