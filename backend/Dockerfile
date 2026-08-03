FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/ebku ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/ebku /app/ebku
RUN mkdir -p /backups && chown -R app:app /app /backups
USER app
ENV BACKUP_DIR=/backups
EXPOSE 8080
ENTRYPOINT ["/app/ebku"]
