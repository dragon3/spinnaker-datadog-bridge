FROM golang:1.11.4-alpine3.8 AS builder

RUN apk add --update git
RUN go get -u github.com/golang/dep/cmd/dep
WORKDIR /go/src/github.com/DataDog/spinnaker-datadog-bridge
COPY . .

RUN dep ensure -v --vendor-only
RUN go build -o /spinnaker-dd-bridge ./cmd/spinnaker-dd-bridge


FROM alpine:3.8

RUN apk add --update ca-certificates
WORKDIR /usr/local/bin
COPY --from=builder /spinnaker-dd-bridge .

CMD ["spinnaker-dd-bridge"]
