FROM registry.access.redhat.com/ubi9/go-toolset:1.23.9-1751282290@sha256:94ed5700c85822759e23cb1ddfa42f2485a56e709106e7060905588f62c46f22 as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:e12131db2e2b6572613589a94b7f615d4ac89d94f859dad05908aeb478fb090f
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
