FROM golang:alpine@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c AS build

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
