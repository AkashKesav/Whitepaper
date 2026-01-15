# NATS Service Configuration for Railway

FROM nats:2.10-alpine

# Expose ports for NATS
EXPOSE 4222 8222

# Health check via monitoring port
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --spider -q http://localhost:8222/healthz || exit 1

# Run with JetStream enabled
CMD ["--jetstream", "--http_port", "8222"]
