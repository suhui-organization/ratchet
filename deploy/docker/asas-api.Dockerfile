# ASAS 控制面镜像。构建上下文 = 仓库根目录：
#   docker build -f deploy/docker/asas-api.Dockerfile -t asas-api:<tag> .
#
# 只用标准库：镜像里没有 pip install，也就没有依赖漂移和供应链面。
FROM python:3.12-slim

WORKDIR /app
COPY service/src/ratchet_service /app/ratchet_service

ENV PYTHONPATH=/app \
    ASAS_DB=/data/asas.db \
    ASAS_PORT=8080 \
    ASAS_VERSION=dev

RUN useradd -u 10001 -m asas && mkdir -p /data && chown -R asas:asas /data /app
USER asas

EXPOSE 8080
CMD ["python3", "-m", "ratchet_service.server"]
