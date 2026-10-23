# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM gcr.io/distroless/static-debian12:nonroot AS api
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]

FROM gcr.io/distroless/static-debian12:nonroot AS worker
COPY --from=build /out/worker /worker
ENTRYPOINT ["/worker"]

FROM oven/bun:1 AS web-build
WORKDIR /src
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web ./
RUN DATABASE_URL=build BETTER_AUTH_SECRET=build BETTER_AUTH_URL=http://localhost bun run build

FROM oven/bun:1 AS web-deps
WORKDIR /src
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile --production

FROM node:24-alpine AS web
WORKDIR /app
ENV NODE_ENV=production
COPY --from=web-deps /src/node_modules ./node_modules
COPY --from=web-build /src/build ./build
COPY web/package.json ./
EXPOSE 3000
CMD ["node", "build"]
