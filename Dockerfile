# ---------- build stage ----------
FROM golang:1.25-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

# Сначала копируем только модульные файлы — слой кэшируется,
# пока не меняются зависимости.
COPY go.mod go.sum* ./
RUN go mod download || go mod tidy

COPY . .

# CGO_ENABLED=0 => статический бинарь, работает в scratch/alpine без libc.
# -ldflags "-s -w" убирает отладочную информацию, уменьшая размер.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/app ./cmd/app

# ---------- runtime stage ----------
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 appuser

COPY --from=builder /bin/app /bin/app

USER appuser
EXPOSE 8080
ENTRYPOINT ["/bin/app"]
