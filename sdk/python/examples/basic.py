"""
Basic example using the RMK Python SDK
"""

import asyncio
from rmk import RMKClient, NodeType, RelationshipType


def main():
    """Synchronous example."""
    with RMKClient(base_url="http://localhost:9090") as client:
        # Login
        print("Logging in...")
        auth = client.login("user", "password")
        print(f"Logged in as: {auth.username} (role: {auth.role})")

        # Store a memory
        print("\n--- Storing memory ---")
        memory = client.memory_store(
            namespace="user_123",
            content="Claude Desktop is an AI assistant that can use MCP tools",
            node_type=NodeType.FACT,
            tags=["ai", "mcp", "claude"],
        )
        print(f"Stored memory: {memory.uid}")

        # Search memories
        print("\n--- Searching memories ---")
        results = client.memory_search(
            namespace="user_123",
            query="Claude Desktop",
            limit=5,
        )
        print(f"Found {results.count} memories")
        for node in results.results:
            print(f"  - {node.get('name', 'N/A')}: {node.get('description', 'N/A')}")

        # Chat consultation
        print("\n--- Chat consultation ---")
        chat = client.chat_consult(
            namespace="user_123",
            message="What do you know about Claude Desktop?",
        )
        print(f"Response: {chat.response}")

        # Create an entity
        print("\n--- Creating entity ---")
        entity = client.entity_create(
            namespace="user_123",
            name="Claude",
            entity_type="Person",
            description="AI assistant by Anthropic",
            relationships=[RelationshipType.WORKS_ON],
        )
        print(f"Created entity: {entity.uid}")

        # List available tools
        print("\n--- Available tools ---")
        tools = client.tools_list()
        print(f"Total tools: {len(tools.tools)}")
        for tool in tools.tools:
            print(f"  - {tool.get('name', 'N/A')}: {tool.get('description', 'N/A')}")


async def async_main():
    """Asynchronous example."""
    async with RMKClient(base_url="http://localhost:9090") as client:
        # Login
        print("Logging in...")
        auth = await client.alogin("user", "password")
        print(f"Logged in as: {auth.username} (role: {auth.role})")

        # Store a memory
        print("\n--- Storing memory ---")
        memory = await client.amemory_store(
            namespace="user_123",
            content="Claude Desktop is an AI assistant that can use MCP tools",
            node_type=NodeType.FACT,
            tags=["ai", "mcp", "claude"],
        )
        print(f"Stored memory: {memory.uid}")

        # Search memories
        print("\n--- Searching memories ---")
        results = await client.amemory_search(
            namespace="user_123",
            query="Claude Desktop",
            limit=5,
        )
        print(f"Found {results.count} memories")


if __name__ == "__main__":
    # Run sync example
    main()

    # Or run async example
    # asyncio.run(async_main())
