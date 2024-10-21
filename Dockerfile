FROM quay.io/app-sre/golang:1.23.1 as builder
WORKDIR /build
COPY . .
RUN make build

FROM registry.access.redhat.com/ubi8-minimal
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
