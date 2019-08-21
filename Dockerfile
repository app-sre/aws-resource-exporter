FROM registry.centos.org/centos/centos:7

COPY aws-resource-exporter /bin/aws-resource-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resource-exporter" ]
