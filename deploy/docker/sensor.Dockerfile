# ASAS 传感器镜像 = ratchet CLI + Python 交付侧 + 上报脚本。
#
# 为什么三样要在一起：`deliver` 会调用 Python 侧出凭据与自查，
# 而传感器要在一个容器里把"采集→出证→上报"整条走完。
#
# 构建上下文 = 仓库根目录：
#   docker build -f deploy/docker/sensor.Dockerfile -t asas-sensor:<tag> .
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/ratchet ./cmd/ratchet

FROM python:3.12-slim

RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/ratchet /usr/local/bin/ratchet
COPY service/src/ratchet_service /app/ratchet_service
COPY deploy/sensor/sensor.sh /usr/local/bin/sensor.sh
RUN chmod 0755 /usr/local/bin/sensor.sh \
    && useradd -u 10001 -m sensor \
    && mkdir -p /data /scan && chown -R sensor:sensor /data /scan

ENV PYTHONPATH=/app \
    ASAS_API=http://asas-api.asas.svc:8080 \
    SCAN_HOME=/scan \
    OUT=/data/sensor

USER sensor
ENTRYPOINT ["/usr/local/bin/sensor.sh"]
