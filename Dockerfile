FROM registry.access.redhat.com/ubi9/go-toolset:1.25.3-1763633888@sha256:71f121a44270e46a17a548d6b92096a1a0fb56db44927ad318d095177d6dce4b as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:61d5ad475048c2e655cd46d0a55dfeaec182cc3faa6348cb85989a7c9e196483
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
