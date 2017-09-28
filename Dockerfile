# This Dockerfile builds an image for a client_golang example.
#
# Use as (from the root for the client_golang repository):
#    docker build -t osrg/taskmonior .

# Builder image, where we build the example.
FROM golang:1.9.0 AS builder
WORKDIR /go/src/github.com/osrg/task-monitor
COPY . .
RUN go get -d
RUN CGO_ENABLED=0 GOOS=linux go build -a -tags netgo -ldflags '-w'

# Final image.
FROM alpine
LABEL maintainer "Yuji Oshima <yuji.oshima0x3fd@gmail.com>"
COPY --from=builder /go/src/github.com/osrg/task-monitor .
EXPOSE 8080
ENTRYPOINT ["./task-monitor"]
