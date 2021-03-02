# build stage
FROM golang as builder

ENV GO111MODULE=on

WORKDIR /app

COPY go.mod .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o template cmd/template/main.go

# final stage
FROM scratch
COPY --from=builder /app/template /template/
COPY --from=builder /app/config.toml /template/
EXPOSE 8080
ENTRYPOINT ["/app/template"]