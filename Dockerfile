FROM registry.access.redhat.com/ubi9/go-toolset:1.25.8-1776644338@sha256:1e1c89558f8bf86db3d88e5d5de0b6bd396ef948749a2c5d6a752ea46f35d4db as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:175bafd5bc7893540ed6234bb979acfe3574fd6570e6762bbc527c757f854cea
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
