FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /proxy .
FROM alpine:3.23
RUN apk add --no-cache ca-certificates
COPY --from=build /proxy /usr/local/bin/proxy
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/proxy"]
