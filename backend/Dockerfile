FROM golang:alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build

WORKDIR /app

RUN apk add --no-cache ca-certificates

ENV GO111MODULE=on CGO_ENABLED=0 GOOS=linux

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN go build -o ./out/executable .


FROM scratch

COPY --from=build /app/out/executable /executable
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

USER 65534:65534

EXPOSE 8080

ENTRYPOINT ["/executable"]
