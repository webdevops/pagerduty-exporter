#############################################
# Build
#############################################
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build

RUN apk upgrade --no-cache --force
RUN apk add --update build-base make git

WORKDIR /go/src/github.com/webdevops/pagerduty-exporter

# Compile
COPY . .
RUN make test
RUN make build # warmup
ARG TARGETOS TARGETARCH
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} make build

#############################################
# Test
#############################################
FROM gcr.io/distroless/static AS test
USER 0:0
WORKDIR /app
COPY --from=build /go/src/github.com/webdevops/pagerduty-exporter/pagerduty-exporter .
RUN ["./pagerduty-exporter", "--help"]

#############################################
# Final
#############################################
FROM gcr.io/distroless/static AS final-static
ENV LOG_JSON=1
WORKDIR /
COPY --from=test /app .
USER 1000:1000
ENTRYPOINT ["/pagerduty-exporter"]
