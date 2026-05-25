FROM registry.access.redhat.com/ubi9/go-toolset:1.26.2-1779719578@sha256:699075dcb578309fecb632f2e7b680d58a27eb5666d2dc60366c892a81127b5d as builder
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
