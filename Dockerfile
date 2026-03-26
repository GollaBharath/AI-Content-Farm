FROM golang:1.25-alpine AS build

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api

FROM alpine:3.20
RUN apk add --no-cache ffmpeg ca-certificates curl python3 py3-pip \
	&& pip3 install --no-cache-dir --break-system-packages --upgrade yt-dlp
WORKDIR /srv
COPY --from=build /bin/api /usr/local/bin/api
RUN mkdir -p /srv/data
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
