#!/bin/bash
# scripts/check-core-deps.sh
# Check core package dependencies against whitelist

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
WHITELIST_FILE="$PROJECT_ROOT/scripts/core-deps-whitelist.txt"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🔍 Checking core package dependencies..."

# Get actual dependencies
ACTUAL_DEPS=$(cd "$PROJECT_ROOT" && go list -f '{{.Imports}}' ./pkg/core/... | tr -d '[]' | tr ' ' '\n' | sort | uniq | grep -v '/pkg/core/' | grep -v '^$')

# Read whitelist
if [[ ! -f "$WHITELIST_FILE" ]]; then
    echo -e "${RED}❌ Whitelist file not found: $WHITELIST_FILE${NC}"
    exit 1
fi

WHITELISTED_DEPS=$(grep -v '^#' "$WHITELIST_FILE" | grep -v '^$' | sort)

# Find violations (dependencies not in whitelist)
VIOLATIONS=$(comm -23 <(echo "$ACTUAL_DEPS") <(echo "$WHITELISTED_DEPS"))

# Find unused whitelist entries (for cleanup)
UNUSED=$(comm -13 <(echo "$ACTUAL_DEPS") <(echo "$WHITELISTED_DEPS"))

if [[ -n "$VIOLATIONS" ]]; then
    echo -e "${RED}❌ CORE DEPENDENCY VIOLATIONS DETECTED:${NC}"
    echo "$VIOLATIONS" | while read -r dep; do
        echo -e "  ${RED}✗${NC} $dep"
    done
    echo ""
    echo -e "${RED}🚫 ARCHITECTURAL BOUNDARY VIOLATION${NC}"
    echo -e "${YELLOW}The core package must not import implementation dependencies directly.${NC}"
    echo ""
    
    # Provide specific guidance based on dependency types
    echo "$VIOLATIONS" | while read -r dep; do
        case "$dep" in
            "database/sql"|*"/sql"|*"mysql"*|*"postgres"*|*"sqlite"*)
                echo -e "${YELLOW}🗄️  Database detected: Create a Repository interface in core${NC}"
                ;;
            "net/http"|*"gin"*|*"echo"*|*"fiber"*|*"mux"*)
                echo -e "${YELLOW}🌐 HTTP framework detected: Create an HTTPHandler interface in core${NC}"
                ;;
            *"redis"*|*"memcache"*|*"etcd"*)
                echo -e "${YELLOW}🔄 Cache/KV store detected: Create a Cache interface in core${NC}"
                ;;
            *"grpc"*|*"protobuf"*)
                echo -e "${YELLOW}📡 RPC framework detected: Create a ServiceClient interface in core${NC}"
                ;;
            *"kafka"*|*"rabbitmq"*|*"nats"*)
                echo -e "${YELLOW}📨 Message queue detected: Create a MessageBroker interface in core${NC}"
                ;;
            "os"|"io/ioutil")
                echo -e "${YELLOW}💾 File system detected: Use existing filesystem.Interface${NC}"
                ;;
        esac
    done
    echo ""
    
    echo -e "${YELLOW}🤖 AI AGENT GUIDANCE:${NC}"
    echo -e "${YELLOW}1. DO NOT add these dependencies to the whitelist${NC}"
    echo -e "${YELLOW}2. DO NOT import concrete implementations in core${NC}"
    echo -e "${YELLOW}3. INSTEAD: Create an interface in core and use dependency injection${NC}"
    echo ""
    echo -e "${YELLOW}📋 PROPER PATTERN:${NC}"
    echo -e "${YELLOW}   // In core package - define interface${NC}"
    echo -e "${YELLOW}   type MyService interface {${NC}"
    echo -e "${YELLOW}       DoSomething(ctx context.Context) error${NC}"
    echo -e "${YELLOW}   }${NC}"
    echo ""
    echo -e "${YELLOW}   // In plugin/implementation package - implement interface${NC}"
    echo -e "${YELLOW}   type ConcreteMyService struct { /* ... */ }${NC}"
    echo -e "${YELLOW}   func (s *ConcreteMyService) DoSomething(ctx context.Context) error { /* ... */ }${NC}"
    echo ""
    echo -e "${YELLOW}   // In main/initialization - inject dependency${NC}"
    echo -e "${YELLOW}   service := &ConcreteMyService{}${NC}"
    echo -e "${YELLOW}   core.SetMyService(service)${NC}"
    echo ""
    echo -e "${YELLOW}📚 EXISTING EXAMPLES:${NC}"
    echo -e "${YELLOW}   - pkg/osshim/filesystem/filesystem.Interface (for file operations)${NC}"
    echo -e "${YELLOW}   - pkg/osshim/httpclient/client.Interface (for HTTP operations)${NC}"
    echo -e "${YELLOW}   - pkg/core/registry/registry.Interface (for repository access)${NC}"
    echo ""
    exit 1
fi

if [[ -n "$UNUSED" ]]; then
    echo -e "${YELLOW}⚠️  Unused whitelist entries (consider cleanup):${NC}"
    echo "$UNUSED" | while read -r dep; do
        echo -e "  ${YELLOW}?${NC} $dep"
    done
    echo ""
fi

echo -e "${GREEN}✅ All core dependencies are whitelisted${NC}"