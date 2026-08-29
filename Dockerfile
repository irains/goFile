FROM golang:1.19.13-alpine3.18 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/goFile .

FROM alpine:3.18
ARG GOFILE_UID=10001
ARG GOFILE_GID=10001

RUN addgroup -S -g "${GOFILE_GID}" gofile \
    && adduser -S -D -H -u "${GOFILE_UID}" -G gofile gofile \
    && mkdir -p /app /data \
    && chown gofile:gofile /data

COPY --from=builder /out/goFile /app/goFile
RUN chmod 0555 /app/goFile

# Templates are embedded in the binary. /data is the only persistent writable path.
WORKDIR /data
USER gofile:gofile
VOLUME ["/data"]
EXPOSE 8089

ENTRYPOINT ["/app/goFile"]
CMD ["-host", "0.0.0.0", "-port", "8089", "-path", "/data", "-cookie-secure"]
