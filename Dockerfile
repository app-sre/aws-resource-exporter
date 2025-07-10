FROM registry.access.redhat.com/ubi9/go-toolset:1.24.4-1752083840@sha256:6fd64cd7f38a9b87440f963b6c04953d04de65c35b9672dbd7f1805b0ae20d09 as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:11db23b63f9476e721f8d0b8a2de5c858571f76d5a0dae2ec28adf08cbaf3652
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
