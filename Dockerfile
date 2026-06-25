FROM registry.access.redhat.com/ubi9/go-toolset:1.26.3-1782377916@sha256:17c888d75753f128f6cbdc5587932c3abd2632ca8e0931aa27b9a60c7a75ac62 as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:44bc70ef6e6ea9a70e353be97f4722e10358d09fbb9494ca943b2a641049690e
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
