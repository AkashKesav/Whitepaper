# Qdrant Vector Database for Railway

FROM qdrant/qdrant:latest

# Expose HTTP and gRPC ports
EXPOSE 6333 6334

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD timeout 3 bash -c 'cat < /dev/null > /dev/tcp/localhost/6333' || exit 1

# Default command
CMD ["./qdrant"]
