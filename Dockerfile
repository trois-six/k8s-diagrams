FROM golang

RUN mkdir /diagram
 
COPY . /diagram
WORKDIR /diagram

RUN make build 
ENTRYPOINT ["/diagram/k8s-diagrams"]

