FROM registry.access.redhat.com/ubi9/go-toolset:1.24.4-1755074415@sha256:81d9cb51feb0b56e3d87a73c2237bebaa80b28e4d5791c1da1140eae50225e2f as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:8d905a93f1392d4a8f7fb906bd49bf540290674b28d82de3536bb4d0898bf9d7
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
