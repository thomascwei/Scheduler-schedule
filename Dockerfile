# Build stage
FROM golang:1.16-alpine3.13 AS builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN go build -o main .

# Run stage
FROM alpine:3.13
RUN mkdir /app
ADD ./config /app/config
COPY --from=builder /build/main /app
WORKDIR /app
EXPOSE 9568
CMD ["./main"]
