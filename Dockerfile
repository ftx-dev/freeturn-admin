# Фронт → Go-бинарник со встроенным фронтом → distroless (без оболочки).
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /freeturn-admin ./cmd/freeturn-admin && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /freeturn-admin /freeturn-admin
# Том создаётся из этой папки и наследует владельца: процесс без root может писать БД и мастер-ключ.
COPY --from=build --chown=65532:65532 /out/data /data
ENV FTA_DATA_DIR=/data FTA_LISTEN=:8080
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/freeturn-admin"]
