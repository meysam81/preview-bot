FROM golang:1.24 AS builder

WORKDIR /app

ENV GO111MODULE=on \
    CGO_ENABLED=0

COPY . .

RUN go mod tidy && go mod vendor && go build -o preview-bot

FROM scratch AS final

ENV GIN_MODE=release

COPY --from=builder /app/preview-bot /preview-bot

ENTRYPOINT ["/preview-bot"]
