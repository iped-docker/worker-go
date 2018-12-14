FROM golang:alpine as builder
WORKDIR /go/src/app
COPY . .
RUN CGO_ENABLED=0 go build -o /go/bin/app .
FROM 192.168.2.191:5001/ipeddocker/iped:3.15
COPY --from=builder /go/bin/app /app
EXPOSE 80
CMD ["/app"]
