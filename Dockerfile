# syntax=docker/dockerfile:1

# Build stage: Go + CGO (mattn/go-sqlite3 needs a C compiler).
FROM golang:1.23-bookworm AS build
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY sqlui/ sqlui/
COPY example/ example/
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/sqlui-example ./example

# Runtime stage: minimal Debian (glibc for the CGO binary).
FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/sqlui-example /app/sqlui-example
ENV SQLUI_ADDR=:8081 \
    SQLUI_DB=/data/demo.db
VOLUME /data
EXPOSE 8081
CMD ["/app/sqlui-example"]
