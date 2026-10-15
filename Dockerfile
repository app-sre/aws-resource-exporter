FROM registry.access.redhat.com/ubi9/go-toolset:1.26.7-1792010711@sha256:60fd6fb2998c40ad0d6561c34e83588b986a0cf7771f64e5e5c295ba87101f9b as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:c4c285d42941ebb2bd4da878479008e1e4536bb599aad1dbe568f89de9053661
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
