FROM golang:1.17 AS build-env

ARG gopkg=k8s.io/perf-tests/util-images/load-pods

ADD ["cmd", "/go/src/$gopkg/cmd"]
ADD ["go.mod", "/go/src/$gopkg"]
ADD ["go.sum", "/go/src/$gopkg"]

WORKDIR /go/src/$gopkg
RUN CGO_ENABLED=0 go build -o /workspace/load-pods ./cmd

FROM scratch
WORKDIR /
COPY --from=build-env /workspace/load-pods /load-pods
ENTRYPOINT ["/load-pods"]

