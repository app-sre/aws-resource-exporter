##
## Build
##

FROM golang:1.19-buster AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /main -ldflags="-s -w" && \
    chmod +x /main

##
## Deploy
##

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /main /aws-resource-exporter

EXPOSE     9115
ENTRYPOINT ["/aws-resource-exporter"]

