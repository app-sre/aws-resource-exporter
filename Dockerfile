FROM registry.access.redhat.com/ubi9/go-toolset:1.26.7-1791275853@sha256:890b54e8d329f33f094ab17881bd37e20b80a54cc39141637310911d8dd9c575 as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:5ed244b62bbf4095080144d9d35eb8fcd3d39a9801f94aadd63b9d10978a01ae
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
