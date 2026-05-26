FROM registry.access.redhat.com/ubi9/go-toolset:1.26.2-1779769584@sha256:a82d974dae02330d0669fb0a5ced2ae498bd1bd708359d61493b9fb0dc0748eb as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:457cb288730d29e2a59823b3b56341466fa94f3158e399c5ab2c4409fee0c562
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
