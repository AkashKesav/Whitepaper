/**
 * Basic example using the RMK TypeScript SDK
 */

import { RMKClient } from "../src/client.js";

async function main() {
  // Initialize client
  const client = new RMKClient({
    baseURL: "http://localhost:9090",
    timeout: 30000,
  });

  try {
    // Login
    console.log("Logging in...");
    const auth = await client.login("user", "password");
    console.log("Logged in as:", auth.username);

    // Store a memory
    console.log("\n--- Storing memory ---");
    const memory = await client.memoryStore({
      namespace: "user_123",
      content: "Claude Desktop is an AI assistant that can use MCP tools",
      nodeType: "Fact",
      tags: ["ai", "mcp", "claude"],
    });
    console.log("Stored memory:", memory);

    // Search memories
    console.log("\n--- Searching memories ---");
    const results = await client.memorySearch({
      namespace: "user_123",
      query: "Claude Desktop",
      limit: 5,
    });
    console.log(`Found ${results.count} memories:`, results.results);

    // Chat consultation
    console.log("\n--- Chat consultation ---");
    const chat = await client.chatConsult({
      namespace: "user_123",
      message: "What do you know about Claude Desktop?",
    });
    console.log("Response:", chat.response);

    // Create an entity
    console.log("\n--- Creating entity ---");
    const entity = await client.entityCreate({
      namespace: "user_123",
      name: "Claude",
      entityType: "Person",
      description: "AI assistant by Anthropic",
      relationships: [
        { type: "WORKS_ON", target: "anthropic" },
      ],
    });
    console.log("Created entity:", entity);

    // List available tools
    console.log("\n--- Available tools ---");
    const tools = await client.toolsList();
    console.log(`Total tools: ${tools.tools.length}`);
    tools.tools.forEach((tool) => {
      console.log(`  - ${tool.name}: ${tool.description}`);
    });

  } catch (error) {
    console.error("Error:", error);
  }
}

main();
