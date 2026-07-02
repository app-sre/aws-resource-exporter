FROM registry.access.redhat.com/ubi9/go-toolset:1.26.4-1782980183@sha256:b2bc0c6c92c56dce575ed1b2dec1ec9f8b345a80ef5e74d9bfd530b6b354feeb as builder
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
