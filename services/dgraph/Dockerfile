# DGraph Service Configuration for Railway
# This uses the official DGraph image with Zero and Alpha combined

FROM dgraph/standalone:latest

# Expose ports for gRPC and HTTP
EXPOSE 8080 9080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Default command uses standalone mode
CMD ["dgraph", "standalone"]
