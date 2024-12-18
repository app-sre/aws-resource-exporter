FROM registry.access.redhat.com/ubi9/go-toolset:1.21.13-2.1727893526@sha256:fd41c001abc243076cc28b63c409ae6d9cbcad401c8124fb67d20fe57a2aa63a as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:daa61d6103e98bccf40d7a69a0d4f8786ec390e2204fd94f7cc49053e9949360
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
