FROM registry.access.redhat.com/ubi9/go-toolset:1.24.4-1753853351@sha256:3ce6311380d5180599a3016031a9112542d43715244816d1d0eabc937952667b as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:aaf57d07d6e192bcb72190f9841f41501b8b3dffdc5e60db2f7a668c2513ab6b
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
