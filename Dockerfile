FROM registry.access.redhat.com/ubi9/go-toolset:1.26.5-1785443561@sha256:0b0dd6f4ee311854a6b8e18b3bc52e7aa6b66f935a16c8f9bda173353ab16c64 as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:48fa5d8cda7fc00d270d8747c3eaa54ae196f0820d8540074a9c8c61d5e3056f
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
