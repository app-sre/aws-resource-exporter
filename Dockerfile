FROM registry.access.redhat.com/ubi9/go-toolset:1.26.10-1793736700@sha256:c2bd0309bf78ae317024fdfc5602a31704e0dba9572f6d4b0bd4b681033dc965 as builder
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
