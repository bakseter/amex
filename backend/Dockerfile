FROM golang:alpine AS build

ENV GO111MODULE=on CGO_ENABLED=0 GOOS=linux

WORKDIR /app

RUN apk add --no-cache ca-certificates

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
