FROM registry.access.redhat.com/ubi9/go-toolset:1.23.6-1745328278@sha256:8a634d63713a073d7a1e086a322e57b148eef9620834fc8266df63d9294dff1b as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:046d27096a14b9c8b6e2bdaae5ef77da72d6bb27634f3e23fce0589aea8ff269
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
