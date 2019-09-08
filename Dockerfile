FROM golang:1.13 as build

WORKDIR /go/src/github.com/webdevops/pagerduty-exporter

# Get deps (cached)
COPY ./go.mod /go/src/github.com/webdevops/pagerduty-exporter
COPY ./go.sum /go/src/github.com/webdevops/pagerduty-exporter
RUN go mod download

# Compile
COPY ./ /go/src/github.com/webdevops/pagerduty-exporter
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o /pagerduty-exporter \
    && chmod +x /pagerduty-exporter
RUN /pagerduty-exporter --help

#############################################
# FINAL IMAGE
#############################################
FROM gcr.io/distroless/static
COPY --from=build /pagerduty-exporter /
USER 1000
ENTRYPOINT ["/pagerduty-exporter"]
