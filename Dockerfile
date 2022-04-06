FROM golang:1.17-alpine as build

RUN apk upgrade --no-cache --force
RUN apk add --update build-base make git

WORKDIR /go/src/github.com/webdevops/pagerduty-exporter

# Compile
COPY ./ /go/src/github.com/webdevops/pagerduty-exporter
RUN make dependencies
RUN make build
RUN ./pagerduty-exporter --help

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/static
ENV LOG_JSON=1
COPY --from=build /go/src/github.com/webdevops/pagerduty-exporter/pagerduty-exporter /
USER 1000:1000
ENTRYPOINT ["/pagerduty-exporter"]
