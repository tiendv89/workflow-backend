# syntax=docker/dockerfile:1
FROM golang:1.25-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/api-service ./cmd

# --- runtime stage ---
FROM debian:bookworm-slim AS runtime

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/api-service /usr/local/bin/api-service
COPY --from=build /src/configs/config.yaml /configs/config.yaml

EXPOSE 8081

ENTRYPOINT ["api-service", "-c", "/configs/config.yaml", "api"]
