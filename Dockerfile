FROM golang AS build

WORKDIR /diagram
COPY . ./
RUN make build

FROM alpine

WORKDIR /
COPY --from=build /diagram/k8s-diagrams /usr/local/bin/k8s-diagrams
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/k8s-diagrams"]
