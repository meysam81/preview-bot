FROM golang:1.25@sha256:699337d620559a59b4a2bb298ad59611e535d2ee755a34cf2d2a98f37578dc80 AS builder

WORKDIR /app

ENV GO111MODULE=on \
    CGO_ENABLED=0

COPY . .

RUN go mod tidy && go build -o preview-bot

FROM scratch AS final

ENV ASSETS_DIR=/

COPY --from=builder /app/preview-bot /preview-bot
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY *.md.tpl /

ENTRYPOINT ["/preview-bot"]
