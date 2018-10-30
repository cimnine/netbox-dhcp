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
    -o netbox-dhcp-linux netbox-dhcp.go

# runner
FROM alpine:latest
RUN apk --no-cache add \
    ca-certificates \
    tcpdump
WORKDIR /app/

COPY netbox-dhcp.docker.conf.yaml /etc/netbox-dhcp.conf.yaml

COPY --from=builder /src/netbox-dhcp-linux ./netbox-dhcp
CMD ["./netbox-dhcp"]
