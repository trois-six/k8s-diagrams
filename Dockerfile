FROM alpine

COPY k8s-diagrams /usr/local/bin/k8s-diagrams

ENTRYPOINT ["/usr/local/bin/k8s-diagrams"]

