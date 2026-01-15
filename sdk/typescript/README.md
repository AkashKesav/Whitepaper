# RMK TypeScript/JavaScript SDK

TypeScript and JavaScript SDK for the Reflective Memory Kernel.

## Installation

```bash
npm install @rmk/sdk
```

## Quick Start

```typescript
import { RMKClient } from '@rmk/sdk';

// Initialize client
const client = new RMKClient({
  baseURL: 'http://localhost:9090',
  timeout: 30000,
});

// Login
const auth = await client.login('username', 'password');
console.log('Logged in as:', auth.username);

// Store a memory
const memory = await client.memoryStore({
  namespace: 'user_123',
  content: 'Claude Desktop is an AI assistant that can use MCP tools',
  nodeType: 'Fact',
  tags: ['ai', 'mcp', 'claude'],
});

// Search memories
const results = await client.memorySearch({
  namespace: 'user_123',
  query: 'Claude Desktop',
  limit: 5,
});

// Chat consultation
const chat = await client.chatConsult({
  namespace: 'user_123',
  message: 'What do you know about Claude Desktop?',
});
```

## API Reference

### Memory Tools

- `memoryStore(params)` - Store a memory in the knowledge graph
- `memorySearch(params)` - Search memories
- `memoryDelete(params)` - Delete a memory
- `memoryList(params)` - List memories

### Chat Tools

- `chatConsult(params)` - Consult the memory kernel
- `conversationsList(params)` - List conversations
- `conversationsDelete(params)` - Delete a conversation

### Entity Tools

- `entityCreate(params)` - Create an entity
- `entityUpdate(params)` - Update an entity
- `entityQuery(params)` - Query entities
- `relationshipCreate(params)` - Create a relationship

### Document Tools

- `documentIngest(params)` - Ingest a document
- `documentList(params)` - List documents
- `documentDelete(params)` - Delete a document

### Group Tools

- `groupCreate(params)` - Create a group
- `groupList()` - List groups
- `groupInvite(params)` - Invite a user to a group
- `groupMembers(params)` - List group members
- `groupShareLink(params)` - Create a share link

### Admin Tools

- `adminUsersList()` - List all users (admin only)
- `adminUserUpdate(params)` - Update a user (admin only)
- `adminMetrics()` - Get system metrics (admin only)
- `adminPoliciesList()` - List policies (admin only)
- `adminPoliciesSet(params)` - Create or update a policy (admin only)

## License

MIT
