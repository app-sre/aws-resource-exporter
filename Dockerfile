FROM registry.access.redhat.com/ubi9/go-toolset:1.25.9-1778504036@sha256:2c17ce45c735ad308240139d807eeb22f4499fd90e883634ba5a191779f1ff94 as builder
COPY LICENSE /licenses/LICENSE
WORKDIR /build
RUN git config --global --add safe.directory /build
COPY . .
RUN make build

FROM builder as test
RUN make test

FROM registry.access.redhat.com/ubi9-minimal@sha256:12db9874bd753eb98b1ab3d840e75de5d6842ac0604fbd68c012adefe97140be
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
