FROM golang:1.14 as build-metrics

WORKDIR /metrics
COPY . .

RUN go build -o /build/metrics


FROM alpine:3.12.0

RUN apk --no-cache add libc6-compat

WORKDIR /metrics
COPY --from=build-metrics /metrics/top.sh .
COPY --from=build-metrics /build/metrics ./build/metrics

CMD ["/metrics/build/metrics"]
