FROM public.ecr.aws/docker/library/golang:1.18 as builder
WORKDIR /build
COPY . .
RUN make build

FROM public.ecr.aws/amazonlinux/amazonlinux:latest
COPY --from=builder /build/aws-resource-exporter  /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
