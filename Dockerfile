FROM golang:1.17 as build

WORKDIR /go/src/github.com/webdevops/pagerduty-exporter

# Get deps (cached)
COPY ./go.mod /go/src/github.com/webdevops/pagerduty-exporter
COPY ./go.sum /go/src/github.com/webdevops/pagerduty-exporter
COPY ./Makefile /go/src/github.com/webdevops/pagerduty-exporter
RUN make dependencies

# Compile
COPY ./ /go/src/github.com/webdevops/pagerduty-exporter
RUN make test
RUN make lint
RUN make build
RUN ./pagerduty-exporter --help

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/static
ENV LOG_JSON=1
COPY --from=build /go/src/github.com/webdevops/pagerduty-exporter/pagerduty-exporter /
USER 1000
ENTRYPOINT ["/pagerduty-exporter"]
