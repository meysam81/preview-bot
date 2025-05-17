FROM golang:1.24 AS builder

WORKDIR /app

ENV GO111MODULE=on \
    CGO_ENABLED=0

COPY . .

RUN go mod tidy && go build -o preview-bot

FROM scratch AS final

COPY --from=builder /app/preview-bot /preview-bot
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY *.md.tpl /

ENTRYPOINT ["/preview-bot"]
