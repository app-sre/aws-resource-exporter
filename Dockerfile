FROM registry.access.redhat.com/ubi9/go-toolset:1.26.5-1786351949@sha256:0b471eb04868f3d9d90bf3c668f9c6c7a22cef07474ac9fec067909dfd7dec7c as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:7c372902c8d211db2d25c8277ba534a73b92742a334874dced829a63b0f21221
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
