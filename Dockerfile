FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /shop-admin-bot ./cmd/bot

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 app
USER app
COPY --from=build /shop-admin-bot /usr/local/bin/shop-admin-bot
EXPOSE 8081
ENTRYPOINT ["shop-admin-bot"]
