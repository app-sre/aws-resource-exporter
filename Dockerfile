FROM registry.access.redhat.com/ubi9/go-toolset:1.26.7-1789950433@sha256:15c3098dc4639e8a0e6a1bc77b50964513a1d76dccae4b07b9808101c5addd04 as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:984df0a2b8d9011d419b0ad260b25f6b74b7ce69ab0595b75522e30f8849f27f
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
