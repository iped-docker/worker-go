#using latest processor
ARG IPED_VERSION=processor_3.18.12_2
FROM golang:alpine as builder
WORKDIR /go/src/app
COPY . .
RUN go generate
RUN CGO_ENABLED=0 go build -o /go/bin/app .
FROM ipeddocker/iped:${IPED_VERSION}
ENV IPEDJAR=/root/IPED/iped/iped.jar
COPY --from=builder /go/bin/app /app
EXPOSE 80
CMD ["/app"]
