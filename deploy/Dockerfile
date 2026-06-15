# Single multi-stage Dockerfile parameterized by SERVICE build-arg (go-structure.md §13).
# GO_VERSION must be >= go.mod's "go" directive (currently 1.25.0).
ARG GO_VERSION=1.25
FROM golang:${GO_VERSION}-alpine AS build

ARG SERVICE
WORKDIR /src

# Cache deps.
COPY go.mod go.sum* ./
RUN go mod download || true

COPY . .

# Inject git SHA at build time if available; falls back to "dev".
ARG GIT_SHA=dev
RUN CGO_ENABLED=0 go build \
      -ldflags "-s -w -X main.GitSHA=${GIT_SHA}" \
      -o /out/service \
      ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/service /app/service
# Bake migrations into the image. Needed for `cmd/migrate`; harmless for other
# services. Baking (instead of volume-mount) avoids the distroless-nonroot
# read issue and keeps the migrate image self-contained for production.
COPY --from=build /src/migrations /app/migrations
# Same for configs — each service binary reads from configs/.
COPY --from=build /src/configs /app/configs
WORKDIR /app
USER nonroot:nonroot
ENTRYPOINT ["/app/service"]
