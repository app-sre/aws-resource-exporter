FROM registry.centos.org/centos/centos:7

COPY aws-resources-exporter /bin/aws-resources-exporter

EXPOSE      9115
ENTRYPOINT  [ "/bin/aws-resources-exporter" ]
