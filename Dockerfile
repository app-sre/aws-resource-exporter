FROM registry.access.redhat.com/ubi9/go-toolset:1.24.4-1753221510@sha256:a90b4605b47c396c74de55f574d0f9e03b24ca177dec54782f86cdf702c97dbc as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:6d5a6576c83816edcc0da7ed62ba69df8f6ad3cbe659adde2891bfbec4dbf187
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
