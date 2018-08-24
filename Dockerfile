FROM golang:1.11-alpine3.8 as builder
RUN apk --no-cache add git

WORKDIR /src/
COPY go.mod .
COPY . .

ENV GO111MODULE=on
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o nine-dhcp2-linux .


FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app/

COPY nine-dhcp2.conf.yaml ./

COPY --from=builder /src/nine-dhcp2-linux .
CMD ["./nine-dhcp2-linux"]
