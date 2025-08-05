FROM registry.access.redhat.com/ubi9/go-toolset:1.24.4-1754384957@sha256:198ee8c19c6b152a94a4c58952151c59efa0207f4f39c05c8b3e8a97b2ed5c0d as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:295f920819a6d05551a1ed50a6c71cb39416a362df12fa0cd149bc8babafccff
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
