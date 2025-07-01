FROM registry.access.redhat.com/ubi9/go-toolset:1.23.9-1751375493@sha256:199e6b43a67ab49a616e8ac72b6fc9b810e075510cb967bc01af0bcda393d1e3 as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:383329bf9c4f968e87e85d30ba3a5cb988a3bbde28b8e4932dcd3a025fd9c98c
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
