# syntax=docker/dockerfile:1
FROM oven/bun:1 AS web
WORKDIR /src/web
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web ./
# svelte-adapter-static writes to ../internal/webui/dist
RUN mkdir -p /src/internal/webui && bun run build

FROM golang:1.27-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=web /src/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/lookfar ./cmd/lookfar

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/lookfar /lookfar
EXPOSE 8080
ENTRYPOINT ["/lookfar"]
CMD ["serve"]
