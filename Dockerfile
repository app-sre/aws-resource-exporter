FROM registry.access.redhat.com/ubi9/go-toolset:1.25.3-1767889151@sha256:38d909b4f0b5244bc6dffab499fa3324e2ce878dcc79e3ee85a200655cbba736 as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:83006d535923fcf1345067873524a3980316f51794f01d8655be55d6e9387183
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
