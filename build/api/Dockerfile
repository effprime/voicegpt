FROM golang:latest AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

WORKDIR /app/cmd/api

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:latest  

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/cmd/api/main .

EXPOSE 8080

CMD ["./main"]
