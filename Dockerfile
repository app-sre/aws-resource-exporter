FROM registry.access.redhat.com/ubi9/go-toolset:1.26.7-1790174511@sha256:0a4666f7a4eb0644c97a73cba198eb268691b270d97831822689e7a2088f87be as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:8ebe2ad8fdf3cab3e5a53c1edc69194c98209cfadab24b884f4ad9ebcf7bbbfc
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
