FROM registry.access.redhat.com/ubi9/go-toolset:1.26.4-1783628461@sha256:9748bb0b8e3376e9b155a4db3fbdfbfd20057b449ea9b3aed367b2f967cdfe1b as builder
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
