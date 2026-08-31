FROM node:24-alpine3.23 AS web-builder
WORKDIR /src/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM golang:1.27.0-alpine3.23 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN rm -rf assets/web
COPY --from=web-builder /src/assets/web ./assets/web
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/fileharbor .

FROM alpine:3.22
ARG FILEHARBOR_UID=10001
ARG FILEHARBOR_GID=10001

RUN addgroup -S -g "${FILEHARBOR_GID}" fileharbor \
    && adduser -S -D -H -u "${FILEHARBOR_UID}" -G fileharbor fileharbor \
    && mkdir -p /app /data /state \
    && chown fileharbor:fileharbor /data /state

COPY --from=builder /out/fileharbor /app/fileharbor
RUN chmod 0555 /app/fileharbor

# Templates and built frontend assets are embedded in the binary. /data and /state are persistent writable paths.
WORKDIR /data
USER fileharbor:fileharbor
VOLUME ["/data", "/state"]
EXPOSE 8089

ENTRYPOINT ["/app/fileharbor"]
CMD ["-host", "0.0.0.0", "-port", "8089", "-path", "/data", "-state-dir", "/state", "-allow-insecure-lan"]
