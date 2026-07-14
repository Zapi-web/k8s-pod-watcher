FROM golang:1.26.5-alpine AS builder
WORKDIR /app

COPY go.mod go.sum /app/
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o operator ./cmd/operator/main.go

FROM gcr.io/distroless/static-debian12:latest

USER 65532:65532
WORKDIR /app

COPY --from=builder /app/operator .

CMD [ "./operator" ]