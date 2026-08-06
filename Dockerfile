FROM registry.access.redhat.com/ubi9/go-toolset:1.26.5-1786023237@sha256:5d26ff5606bd6590930e7cfc202b510e3fe2c7a7a1720860f444ab49c45128cb as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:dd334afa72444fa46238fcf9e6bd399245adf746378735348cf84b9dfdca38f1
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
