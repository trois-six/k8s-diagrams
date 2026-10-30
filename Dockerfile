FROM golang:1.27 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /k8s-diagrams .

FROM alpine:3

COPY --from=build /k8s-diagrams /usr/local/bin/k8s-diagrams
USER nobody:nobody
ENTRYPOINT ["/usr/local/bin/k8s-diagrams"]
