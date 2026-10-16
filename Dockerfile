# Сборка идёт на архитектуре раннера, бинарник кросс-компилируется под целевую.
FROM --platform=$BUILDPLATFORM golang:1.26.3-alpine AS build
ARG COMMIT=dev
ARG BUILD_DATE=
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" -o /out/bot ./cmd/bot

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -o /out/bot-verify ./cmd/bot-verify

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata && addgroup -g 10001 bot && adduser -D -u 10001 -G bot bot
WORKDIR /app
COPY --from=build /out/bot /app/bot
COPY --from=build /out/bot-verify /app/bot-verify
# Уведомления о лицензиях едут вместе с бинарником (требование MIT/BSD).
COPY LICENSE THIRD-PARTY-NOTICES.md /app/
VOLUME ["/data"]
ENV DATA_DIR=/data
USER 10001:10001
ENTRYPOINT ["/app/bot"]
