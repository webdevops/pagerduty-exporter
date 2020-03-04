FROM golang:1.14 as build

WORKDIR /go/src/github.com/webdevops/pagerduty-exporter

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.23.8

# Get deps (cached)
COPY ./go.mod /go/src/github.com/webdevops/pagerduty-exporter
COPY ./go.sum /go/src/github.com/webdevops/pagerduty-exporter
RUN go mod download

# Compile
COPY ./ /go/src/github.com/webdevops/pagerduty-exporter
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o /pagerduty-exporter \
    && chmod +x /pagerduty-exporter
RUN /pagerduty-exporter --help
RUN golangci-lint run -D megacheck -E unused,gosimple,staticcheck

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/static
COPY --from=build /pagerduty-exporter /
USER 1000
ENTRYPOINT ["/pagerduty-exporter"]
