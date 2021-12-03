FROM golang:alpine as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN go build -o main .
FROM alpine
RUN mkdir /app
ADD . /app/
COPY --from=builder /build/main /app
WORKDIR /app
EXPOSE 9568
CMD ["./main"]