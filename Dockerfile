FROM registry.access.redhat.com/ubi9/go-toolset:1.25.9-1778054913@sha256:180d433d97773ac90384662ee0f54c3b474f6eeb7219e414a4ca323d1196bb13 as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:8d0a8fb39ec907e8ca62cdd24b62a63ca49a30fe465798a360741fde58437a23
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
