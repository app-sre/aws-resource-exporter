FROM registry.access.redhat.com/ubi9/go-toolset:1.23.9-1749052980@sha256:d4bcb27c15c76c727742a78173ba02a56d9acc695ecc2603a0dc0fc745a75685 as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:f172b3082a3d1bbe789a1057f03883c1113243564f01cd3020e27548b911d3f8
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
