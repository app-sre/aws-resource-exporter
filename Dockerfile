FROM registry.access.redhat.com/ubi9/go-toolset:1.23.6-1747333074@sha256:e0ad156b08e0b50ad509d79513e13e8a31f2812c66e9c48c98cea53420ec2bca as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:f172b3082a3d1bbe789a1057f03883c1113243564f01cd3020e27548b911d3f8
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
