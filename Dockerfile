FROM registry.access.redhat.com/ubi9/go-toolset:1.26.10-1793621864@sha256:5f0dead6a152de46a19f51322ea8e06d0d9f282a4937d663cf5a226cb8744bc8 as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:dfa0eb04abc59221544e74862432496add1f491f915c9b531880f91affc0e12c
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
