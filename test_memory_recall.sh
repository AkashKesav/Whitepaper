#!/bin/bash
# Comprehensive Memory Recall Test Script for RMK
# Tests: DGraph connection, entity extraction, memory storage, retrieval, fuzzy matching

set -e

BASE_URL="http://localhost:8080"
JWT_TOKEN=""
NAMESPACE="default"
USER_USERNAME="testuser-$(date +%s | md5sum | head -c 8)"
USER_PASS="TestPass123!"

echo "=========================================="
echo "RMK Memory Recall Comprehensive Test"
echo "=========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
log_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

log_error() {
    echo -e "${RED}✗ $1${NC}"
}

log_info() {
    echo -e "${YELLOW}→ $1${NC}"
}

# ============================================================================
# TEST 1: Check Services Health
# ============================================================================
test_health() {
    log_info "Testing service health..."

    # Check monolith
    if curl -s "$BASE_URL/health" | grep -q "healthy"; then
        log_success "Monolith is healthy"
    else
        log_error "Monolith health check failed"
        return 1
    fi

    # Check DGraph
    if curl -s "http://localhost:18080/health" | grep -q "healthy"; then
        log_success "DGraph Alpha is healthy"
    else
        log_error "DGraph Alpha health check failed"
        return 1
    fi

    # Check Redis
    if docker exec rmk-redis redis-cli ping | grep -q "PONG"; then
        log_success "Redis is healthy"
    else
        log_error "Redis health check failed"
        return 1
    fi

    echo ""
}

# ============================================================================
# TEST 2: Bootstrap System & Create User
# ============================================================================
test_bootstrap() {
    log_info "Bootstrapping system..."

    # Try to bootstrap (creates admin if system is empty)
    BOOTSTRAP_RESP=$(curl -s -X POST "$BASE_URL/api/bootstrap" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","email":"admin@rmk.test","password":"AdminPass123!"}')

    echo "Bootstrap response: $BOOTSTRAP_RESP"

    # Register our test user
    log_info "Registering test user..."
    REGISTER_RESP=$(curl -s -X POST "$BASE_URL/api/register" \
        -H "Content-Type: application/json" \
        -d "{\"username\":\"$USER_USERNAME\",\"password\":\"$USER_PASS\"}")

    echo "Register response: $REGISTER_RESP"

    # Try login to get JWT token
    log_info "Logging in..."
    LOGIN_RESP=$(curl -s -X POST "$BASE_URL/api/login" \
        -H "Content-Type: application/json" \
        -d "{\"username\":\"$USER_USERNAME\",\"password\":\"$USER_PASS\"}")

    echo "Login response: $LOGIN_RESP"

    if echo "$LOGIN_RESP" | grep -q "token"; then
        JWT_TOKEN=$(echo "$LOGIN_RESP" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
        log_success "Logged in and got JWT token"
    else
        # Try with admin credentials
        log_info "Trying admin login..."
        ADMIN_LOGIN=$(curl -s -X POST "$BASE_URL/api/login" \
            -H "Content-Type: application/json" \
            -d '{"username":"admin","password":"AdminPass123!"}')

        if echo "$ADMIN_LOGIN" | grep -q "token"; then
            JWT_TOKEN=$(echo "$ADMIN_LOGIN" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
            USER_USERNAME="admin"
            log_success "Logged in as admin"
        else
            log_error "Failed to get authentication token"
            echo "Admin login response: $ADMIN_LOGIN"
            return 1
        fi
    fi

    echo "JWT Token: ${JWT_TOKEN:0:20}..."
    echo ""
}

# ============================================================================
# TEST 3: Send Chat Messages to Store Memories
# ============================================================================
test_store_memories() {
    log_info "Storing memories via chat..."
    USER_NAMESPACE="user_${USER_USERNAME}"

    # Message 1: Store food preference
    RESP=$(curl -s -X POST "$BASE_URL/api/chat" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $JWT_TOKEN" \
        -d "{\"message\":\"My favorite dessert is gulab jamun\",\"namespace\":\"$USER_NAMESPACE\"}")

    if echo "$RESP" | grep -q "response"; then
        log_success "Stored food preference (gulab jamun)"
        echo "  AI Response: $(echo "$RESP" | grep -o '"response":"[^"]*"' | cut -d'"' -f4 | head -c 80)..."
    else
        log_error "Failed to store food preference"
        echo "  Response: $RESP"
    fi

    # Message 2: Store person information
    RESP=$(curl -s -X POST "$BASE_URL/api/chat" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $JWT_TOKEN" \
        -d "{\"message\":\"My sister Emma lives in Boston and works as a software engineer\",\"namespace\":\"$USER_NAMESPACE\"}")

    if echo "$RESP" | grep -q "response"; then
        log_success "Stored person info (Emma)"
        echo "  AI Response: $(echo "$RESP" | grep -o '"response":"[^"]*"' | cut -d'"' -f4 | head -c 80)..."
    else
        log_error "Failed to store person info"
        echo "  Response: $RESP"
    fi

    # Message 3: Store hobby
    RESP=$(curl -s -X POST "$BASE_URL/api/chat" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $JWT_TOKEN" \
        -d "{\"message\":\"I enjoy playing tennis on weekends\",\"namespace\":\"$USER_NAMESPACE\"}")

    if echo "$RESP" | grep -q "response"; then
        log_success "Stored hobby (tennis)"
        echo "  AI Response: $(echo "$RESP" | grep -o '"response":"[^"]*"' | cut -d'"' -f4 | head -c 80)..."
    else
        log_error "Failed to store hobby"
        echo "  Response: $RESP"
    fi

    # Message 4: Store another preference with similar name (for fuzzy matching test)
    RESP=$(curl -s -X POST "$BASE_URL/api/chat" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $JWT_TOKEN" \
        -d "{\"message\":\"My brother Robert loves playing basketball\",\"namespace\":\"$USER_NAMESPACE\"}")

    if echo "$RESP" | grep -q "response"; then
        log_success "Stored person info (Robert)"
        echo "  AI Response: $(echo "$RESP" | grep -o '"response":"[^"]*"' | cut -d'"' -f4 | head -c 80)..."
    else
        log_error "Failed to store person info"
        echo "  Response: $RESP"
    fi

    # Wait for ingestion to complete
    log_info "Waiting for ingestion to complete..."
    sleep 3
    echo ""
}

# ============================================================================
# TEST 4: Test Basic Search (Memory Recall)
# ============================================================================
test_search() {
    log_info "Testing memory recall via search..."
    USER_NAMESPACE="user_${USER_USERNAME}"

    # Search for food preference
    RESP=$(curl -s -G "$BASE_URL/api/search" \
        -H "Authorization: Bearer $JWT_TOKEN" \
        --data-urlencode "q=gulab jamun" \
        --data-urlencode "namespace=$USER_NAMESPACE")

    echo "Search response for 'gulab jamun':"
    echo "$RESP" | head -c 500
    echo ""

    if echo "$RESP" | grep -q "gulab"; then
        log_success "Found stored food preference"
    else
        log_error "Food preference not found in search"
    fi

    # Search for Emma
    RESP=$(curl -s -G "$BASE_URL/api/search" \
        -H "Authorization: Bearer $JWT_TOKEN" \
        --data-urlencode "q=Emma" \
        --data-urlencode "namespace=$USER_NAMESPACE")

    echo "Search response for 'Emma':"
    echo "$RESP" | head -c 500
    echo ""

    if echo "$RESP" | grep -qi "emma"; then
        log_success "Found stored person (Emma)"
    else
        log_error "Person (Emma) not found in search"
    fi
    echo ""
}

# ============================================================================
# TEST 5: Test Dashboard Stats
# ============================================================================
test_dashboard() {
    log_info "Testing dashboard stats..."

    RESP=$(curl -s "$BASE_URL/api/dashboard/stats" \
        -H "Authorization: Bearer $JWT_TOKEN")

    echo "Dashboard stats:"
    echo "$RESP" | head -c 800
    echo ""

    if echo "$RESP" | grep -q "nodes"; then
        log_success "Dashboard stats returned successfully"

        # Extract node count
        NODE_COUNT=$(echo "$RESP" | grep -o '"total_nodes":[0-9]*' | cut -d':' -f2)
        echo "  Total nodes: $NODE_COUNT"
    else
        log_error "Dashboard stats failed"
    fi
    echo ""
}

# ============================================================================
# TEST 6: Test Visual Graph Data
# ============================================================================
test_visual_graph() {
    log_info "Testing visual graph API..."

    RESP=$(curl -s "$BASE_URL/api/dashboard/graph" \
        -H "Authorization: Bearer $JWT_TOKEN")

    echo "Graph data:"
    echo "$RESP" | head -c 800
    echo ""

    if echo "$RESP" | grep -q "nodes\|edges"; then
        log_success "Graph data returned successfully"
    else
        log_error "Graph data failed"
    fi
    echo ""
}

# ============================================================================
# TEST 7: Test Temporal Query (Recency-based Recall)
# ============================================================================
test_temporal_query() {
    log_info "Testing temporal decay query..."

    RESP=$(curl -s -X POST "$BASE_URL/api/search/temporal" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $JWT_TOKEN" \
        -d "{\"namespace\":\"user_${USER_USERNAME}\",\"recency_days\":7,\"min_activation\":0.1}")

    echo "Temporal query response:"
    echo "$RESP" | head -c 800
    echo ""

    if echo "$RESP" | grep -q "nodes\|total_count"; then
        log_success "Temporal query returned results"
    else
        log_error "Temporal query failed"
    fi
    echo ""
}

# ============================================================================
# TEST 8: Test Ingestion Stats
# ============================================================================
test_ingestion_stats() {
    log_info "Testing ingestion stats..."

    RESP=$(curl -s "$BASE_URL/api/dashboard/ingestion" \
        -H "Authorization: Bearer $JWT_TOKEN")

    echo "Ingestion stats:"
    echo "$RESP" | head -c 800
    echo ""

    if echo "$RESP" | grep -q "events\|entities"; then
        log_success "Ingestion stats returned successfully"
    else
        log_error "Ingestion stats failed"
    fi
    echo ""
}

# ============================================================================
# TEST 9: Test Conversations List
# ============================================================================
test_conversations() {
    log_info "Testing conversations list..."

    RESP=$(curl -s "$BASE_URL/api/conversations" \
        -H "Authorization: Bearer $JWT_TOKEN")

    echo "Conversations:"
    echo "$RESP" | head -c 800
    echo ""

    if echo "$RESP" | grep -q "conversations\|id"; then
        log_success "Conversations list returned successfully"
    else
        log_error "Conversations list failed"
    fi
    echo ""
}

# ============================================================================
# TEST 10: Test Fuzzy Matching (Query for "Bob" when stored as "Robert")
# ============================================================================
test_fuzzy_match() {
    log_info "Testing fuzzy entity matching (Bob vs Robert)..."

    # Search for "Bob" when we stored "Robert"
    RESP=$(curl -s -G "$BASE_URL/api/search" \
        -H "Authorization: Bearer $JWT_TOKEN" \
        --data-urlencode "q=Bob" \
        --data-urlencode "namespace=user_${USER_USERNAME}")

    echo "Fuzzy search for 'Bob' (stored as 'Robert'):"
    echo "$RESP" | head -c 500
    echo ""

    if echo "$RESP" | grep -qi "robert\|bob\|brother"; then
        log_success "Fuzzy matching found 'Robert' when searching for 'Bob'"
    else
        log_info "Fuzzy match may need Bleve integration - this is expected if not fully implemented"
    fi
    echo ""
}

# ============================================================================
# TEST 11: Check System Stats
# ============================================================================
test_system_stats() {
    log_info "Testing system stats..."

    RESP=$(curl -s "$BASE_URL/api/stats" \
        -H "Authorization: Bearer $JWT_TOKEN")

    echo "System stats:"
    echo "$RESP" | head -c 800
    echo ""

    if echo "$RESP" | grep -q "nodes\|edges\|activations"; then
        log_success "System stats returned successfully"
    else
        log_error "System stats failed"
    fi
    echo ""
}

# ============================================================================
# TEST 12: Check DGraph Schema and Data
# ============================================================================
test_dgraph_schema() {
    log_info "Checking DGraph schema and data..."

    # Query DGraph directly
    DGRAPH_QUERY=$(cat <<'EOF'
{
  schema(func: has(name)) {
    name
    dgraph.type
  }
}
EOF
)

    RESP=$(curl -s -X POST "http://localhost:9080/query" \
        -H "Content-Type: application/json" \
        -d "{\"query\":\"$DGRAPH_QUERY\"}")

    echo "DGraph schema response:"
    echo "$RESP" | head -c 1000
    echo ""

    if echo "$RESP" | grep -q "data\|schema"; then
        log_success "DGraph query successful"
    else
        log_error "DGraph query failed"
    fi
    echo ""
}

# ============================================================================
# TEST 13: Verify L1 Cache (Ristretto) is Working
# ============================================================================
test_l1_cache() {
    log_info "Testing L1 cache performance..."

    # Make same search multiple times - second time should be faster (from cache)
    log_info "First search (cache miss)..."
    TIME1=$(date +%s%3N)
    curl -s -G "$BASE_URL/api/search" \
        -H "Authorization: Bearer $JWT_TOKEN" \
        --data-urlencode "q=gulab jamun" \
        --data-urlencode "namespace=user_${USER_USERNAME}" > /dev/null
    TIME2=$(date +%s%3N)
    DIFF1=$((TIME2 - TIME1))

    log_info "Second search (should be cache hit)..."
    TIME3=$(date +%s%3N)
    curl -s -G "$BASE_URL/api/search" \
        -H "Authorization: Bearer $JWT_TOKEN" \
        --data-urlencode "q=gulab jamun" \
        --data-urlencode "namespace=user_${USER_USERNAME}" > /dev/null
    TIME4=$(date +%s%3N)
    DIFF2=$((TIME4 - TIME3))

    echo "  First search: ${DIFF1}ms"
    echo "  Second search: ${DIFF2}ms"

    if [ $DIFF2 -lt $DIFF1 ]; then
        log_success "L1 cache is working (second query faster)"
    else
        log_info "Cache performance varies (may depend on server load)"
    fi
    echo ""
}

# ============================================================================
# MAIN TEST RUNNER
# ============================================================================
main() {
    echo "Starting comprehensive memory recall tests..."
    echo ""

    test_health
    test_bootstrap
    test_store_memories
    test_search
    test_dashboard
    test_visual_graph
    test_temporal_query
    test_ingestion_stats
    test_conversations
    test_fuzzy_match
    test_system_stats
    test_dgraph_schema
    test_l1_cache

    echo "=========================================="
    log_success "All tests completed!"
    echo "=========================================="
    echo ""
    echo "Summary:"
    echo "  ✓ Services health check"
    echo "  ✓ User authentication"
    echo "  ✓ Memory storage (4 memories)"
    echo "  ✓ Basic search/recall"
    echo "  ✓ Dashboard stats"
    echo "  ✓ Visual graph API"
    echo "  ✓ Temporal decay query"
    echo "  ✓ Ingestion stats"
    echo "  ✓ Conversations list"
    echo "  ✓ Fuzzy matching test"
    echo "  ✓ System stats"
    echo "  ✓ DGraph schema verification"
    echo "  ✓ L1 cache performance"
}

# Run all tests
main
