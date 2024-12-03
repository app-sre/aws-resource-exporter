FROM registry.access.redhat.com/ubi9/go-toolset:9.5-1733160835@sha256:e8e961aebb9d3acedcabb898129e03e6516b99244eb64330e5ca599af9c7aa3d as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:d85040b6e3ed3628a89683f51a38c709185efc3fb552db2ad1b9180f2a6c38be
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
