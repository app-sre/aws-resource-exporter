FROM registry.access.redhat.com/ubi9/go-toolset:1.23.6-1744294473@sha256:8a9e1465ae487fa2b4a94d2baa249a8d59fb3d7e6884a7b22c783f494f4cfb01 as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:ac61c96b93894b9169221e87718733354dd3765dd4a62b275893c7ff0d876869
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
