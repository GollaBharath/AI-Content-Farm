FROM golang:1.22-alpine AS build

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api

FROM alpine:3.20
RUN addgroup -S app && adduser -S app -G app
WORKDIR /srv
COPY --from=build /bin/api /usr/local/bin/api
RUN mkdir -p /srv/data && chown -R app:app /srv
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
