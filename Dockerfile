FROM registry.access.redhat.com/ubi9/go-toolset:1.26.5-1784751462@sha256:5f5c97d7e6d917b8328321bcf2c9d5700de65b72d434ecdbbba6f35aaebaad40 as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:c5478a52c410e71c53839923c83a1480199a1e74ce5736fe3e3a5578dc399102
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
