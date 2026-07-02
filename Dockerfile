# Stage 1: build the Go collector.
FROM golang:1.22-alpine AS go-build
WORKDIR /src
COPY go.mod ./
COPY internal ./internal
COPY cmd ./cmd
RUN CGO_ENABLED=0 go build -o /out/diagkit ./cmd/diagkit

# Stage 2: runtime with Python for the analyzer plus the collector binary.
FROM python:3.12-slim
WORKDIR /app
COPY --from=go-build /out/diagkit /usr/local/bin/diagkit
COPY py /app/py
RUN pip install --no-cache-dir /app/py
WORKDIR /app/py

# Default: run the full collect -> analyze pipeline on the seeded incident.
ENTRYPOINT ["/bin/sh", "-c"]
CMD ["diagkit collect --out - | python -m diagkit_rca analyze -"]
