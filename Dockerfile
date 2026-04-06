FROM registry.access.redhat.com/ubi9/go-toolset:1.25.8-1775437082@sha256:469da742918d6916d441952dd53a760aa2cb5397d672c9143b0dcfa01f9efe91 as builder
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
