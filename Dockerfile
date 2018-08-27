# builder
FROM golang:1.11rc2-alpine3.8 as builder
RUN apk --no-cache add git

WORKDIR /src/

ENV GO111MODULE=on
COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 \
    GOOS=linux \
    go build -a -installsuffix cgo \
    -o nine-dhcp2-linux .

# runner
FROM alpine:latest
RUN apk --no-cache add \
    ca-certificates
WORKDIR /app/

COPY nine-dhcp2.conf.yaml ./

COPY --from=builder /src/nine-dhcp2-linux .
CMD ["./nine-dhcp2-linux"]
