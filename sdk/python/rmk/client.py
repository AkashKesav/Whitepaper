"""
RMK Python SDK

Reflective Memory Kernel SDK for Python
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from typing import Any, AsyncIterator, Optional, Union

import httpx


class RMKError(Exception):
    """Error raised by the RMK SDK."""

    def __init__(self, message: str, status_code: Optional[int] = None, details: Optional[Any] = None):
        super().__init__(message)
        self.status_code = status_code
        self.details = details


class RMKClient:
    """Client for interacting with the RMK API."""

    def __init__(
        self,
        base_url: str = "http://localhost:9090",
        timeout: float = 30.0,
        token: Optional[str] = None,
    ):
        """
        Initialize the RMK client.

        Args:
            base_url: Base URL of the RMK API
            timeout: Request timeout in seconds
            token: Optional authentication token
        """
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.token = token
        self._client: Optional[httpx.Client] = None
        self._async_client: Optional[httpx.AsyncClient] = None

    @property
    def client(self) -> httpx.Client:
        """Get or create the sync HTTP client."""
        if self._client is None:
            self._client = httpx.Client(base_url=self.base_url, timeout=self.timeout)
        return self._client

    @property
    def async_client(self) -> httpx.AsyncClient:
        """Get or create the async HTTP client."""
        if self._async_client is None:
            self._async_client = httpx.AsyncClient(base_url=self.base_url, timeout=self.timeout)
        return self._async_client

    def set_token(self, token: str) -> None:
        """Set the authentication token."""
        self.token = token

    def get_token(self) -> Optional[str]:
        """Get the current token."""
        return self.token

    def clear_token(self) -> None:
        """Clear the authentication token."""
        self.token = None

    def close(self) -> None:
        """Close the HTTP client."""
        if self._client is not None:
            self._client.close()
            self._client = None
        if self._async_client is not None:
            import asyncio

            try:
                loop = asyncio.get_event_loop()
                if loop.is_running():
                    asyncio.create_task(self._async_client.aclose())
                else:
                    loop.run_until_complete(self._async_client.aclose())
            except RuntimeError:
                pass
            self._async_client = None

    async def aclose(self) -> None:
        """Close the async HTTP client."""
        if self._async_client is not None:
            await self._async_client.aclose()
            self._async_client = None
        if self._client is not None:
            self._client.close()
            self._client = None

    def _get_headers(self) -> dict[str, str]:
        """Get request headers with authentication."""
        headers = {"Content-Type": "application/json"}
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"
        return headers

    def _handle_response(self, response: httpx.Response) -> Any:
        """Handle HTTP response."""
        if not response.is_success:
            raise RMKError(
                f"HTTP {response.status_code}: {response.text}",
                status_code=response.status_code,
            )
        return response.json()

    async def _ahandle_response(self, response: httpx.Response) -> Any:
        """Handle async HTTP response."""
        if not response.is_success:
            raise RMKError(
                f"HTTP {response.status_code}: {response.text}",
                status_code=response.status_code,
            )
        return response.json()

    # ========== AUTHENTICATION ==========

    def login(self, username: str, password: str) -> AuthResponse:
        """Login with username and password."""
        response = self.client.post(
            "/api/login",
            json={"username": username, "password": password},
        )
        data = self._handle_response(response)
        self.token = data["token"]
        return AuthResponse(**data)

    async def alogin(self, username: str, password: str) -> AuthResponse:
        """Async login with username and password."""
        response = await self.async_client.post(
            "/api/login",
            json={"username": username, "password": password},
        )
        data = await self._ahandle_response(response)
        self.token = data["token"]
        return AuthResponse(**data)

    def logout(self) -> None:
        """Logout."""
        try:
            self.client.post("/api/logout", headers=self._get_headers())
        finally:
            self.clear_token()

    async def alogout(self) -> None:
        """Async logout."""
        try:
            await self.async_client.post("/api/logout", headers=self._get_headers())
        finally:
            self.clear_token()

    # ========== MEMORY TOOLS ==========

    def memory_store(
        self,
        namespace: str,
        content: str,
        node_type: NodeType,
        name: Optional[str] = None,
        tags: Optional[list[str]] = None,
    ) -> MemoryStoreResult:
        """Store a memory in the knowledge graph."""
        return self._tool_call(
            "memory_store",
            {
                "namespace": namespace,
                "content": content,
                "node_type": node_type.value,
                "name": name,
                "tags": tags or [],
            },
            MemoryStoreResult,
        )

    async def amemory_store(
        self,
        namespace: str,
        content: str,
        node_type: NodeType,
        name: Optional[str] = None,
        tags: Optional[list[str]] = None,
    ) -> MemoryStoreResult:
        """Async store a memory."""
        return await self._atool_call(
            "memory_store",
            {
                "namespace": namespace,
                "content": content,
                "node_type": node_type.value,
                "name": name,
                "tags": tags or [],
            },
            MemoryStoreResult,
        )

    def memory_search(self, namespace: str, query: str, limit: int = 10) -> MemorySearchResult:
        """Search memories."""
        return self._tool_call(
            "memory_search",
            {"namespace": namespace, "query": query, "limit": limit},
            MemorySearchResult,
        )

    async def amemory_search(
        self, namespace: str, query: str, limit: int = 10
    ) -> MemorySearchResult:
        """Async search memories."""
        return await self._atool_call(
            "memory_search",
            {"namespace": namespace, "query": query, "limit": limit},
            MemorySearchResult,
        )

    def memory_delete(self, namespace: str, uid: str) -> None:
        """Delete a memory."""
        self._tool_call("memory_delete", {"namespace": namespace, "uid": uid}, None)

    async def amemory_delete(self, namespace: str, uid: str) -> None:
        """Async delete a memory."""
        await self._atool_call("memory_delete", {"namespace": namespace, "uid": uid}, None)

    def memory_list(
        self, namespace: str, node_type: Optional[NodeType] = None, limit: int = 50, offset: int = 0
    ) -> MemoryListResult:
        """List memories."""
        args: dict[str, Any] = {"namespace": namespace, "limit": limit, "offset": offset}
        if node_type:
            args["node_type"] = node_type.value
        return self._tool_call("memory_list", args, MemoryListResult)

    async def amemory_list(
        self, namespace: str, node_type: Optional[NodeType] = None, limit: int = 50, offset: int = 0
    ) -> MemoryListResult:
        """Async list memories."""
        args: dict[str, Any] = {"namespace": namespace, "limit": limit, "offset": offset}
        if node_type:
            args["node_type"] = node_type.value
        return await self._atool_call("memory_list", args, MemoryListResult)

    # ========== CHAT TOOLS ==========

    def chat_consult(
        self, namespace: str, message: str, conversation_id: Optional[str] = None
    ) -> ChatConsultResult:
        """Consult the memory kernel."""
        args: dict[str, Any] = {"namespace": namespace, "message": message}
        if conversation_id:
            args["conversation_id"] = conversation_id
        return self._tool_call("chat_consult", args, ChatConsultResult)

    async def achat_consult(
        self, namespace: str, message: str, conversation_id: Optional[str] = None
    ) -> ChatConsultResult:
        """Async consult the memory kernel."""
        args: dict[str, Any] = {"namespace": namespace, "message": message}
        if conversation_id:
            args["conversation_id"] = conversation_id
        return await self._atool_call("chat_consult", args, ChatConsultResult)

    def conversations_list(self, namespace: str, limit: int = 20) -> ConversationsListResult:
        """List conversations."""
        return self._tool_call(
            "conversations_list", {"namespace": namespace, "limit": limit}, ConversationsListResult
        )

    async def aconversations_list(self, namespace: str, limit: int = 20) -> ConversationsListResult:
        """Async list conversations."""
        return await self._atool_call(
            "conversations_list", {"namespace": namespace, "limit": limit}, ConversationsListResult
        )

    def conversations_delete(self, namespace: str, conversation_id: str) -> None:
        """Delete a conversation."""
        self._tool_call(
            "conversations_delete", {"namespace": namespace, "conversation_id": conversation_id}, None
        )

    async def aconversations_delete(self, namespace: str, conversation_id: str) -> None:
        """Async delete a conversation."""
        await self._atool_call(
            "conversations_delete", {"namespace": namespace, "conversation_id": conversation_id}, None
        )

    # ========== ENTITY TOOLS ==========

    def entity_create(
        self,
        namespace: str,
        name: str,
        entity_type: str,
        description: Optional[str] = None,
        relationships: Optional[list[Relationship]] = None,
        attributes: Optional[dict[str, str]] = None,
    ) -> EntityCreateResult:
        """Create an entity."""
        args: dict[str, Any] = {
            "namespace": namespace,
            "name": name,
            "entity_type": entity_type,
        }
        if description:
            args["description"] = description
        if relationships:
            args["relationships"] = [r.to_dict() for r in relationships]
        if attributes:
            args["attributes"] = attributes
        return self._tool_call("entity_create", args, EntityCreateResult)

    async def aentity_create(
        self,
        namespace: str,
        name: str,
        entity_type: str,
        description: Optional[str] = None,
        relationships: Optional[list[Relationship]] = None,
        attributes: Optional[dict[str, str]] = None,
    ) -> EntityCreateResult:
        """Async create an entity."""
        args: dict[str, Any] = {
            "namespace": namespace,
            "name": name,
            "entity_type": entity_type,
        }
        if description:
            args["description"] = description
        if relationships:
            args["relationships"] = [r.to_dict() for r in relationships]
        if attributes:
            args["attributes"] = attributes
        return await self._atool_call("entity_create", args, EntityCreateResult)

    def entity_update(
        self,
        namespace: str,
        uid: str,
        name: Optional[str] = None,
        description: Optional[str] = None,
        attributes: Optional[dict[str, str]] = None,
    ) -> EntityUpdateResult:
        """Update an entity."""
        args: dict[str, Any] = {"namespace": namespace, "uid": uid}
        if name:
            args["name"] = name
        if description:
            args["description"] = description
        if attributes:
            args["attributes"] = attributes
        return self._tool_call("entity_update", args, EntityUpdateResult)

    async def aentity_update(
        self,
        namespace: str,
        uid: str,
        name: Optional[str] = None,
        description: Optional[str] = None,
        attributes: Optional[dict[str, str]] = None,
    ) -> EntityUpdateResult:
        """Async update an entity."""
        args: dict[str, Any] = {"namespace": namespace, "uid": uid}
        if name:
            args["name"] = name
        if description:
            args["description"] = description
        if attributes:
            args["attributes"] = attributes
        return await self._atool_call("entity_update", args, EntityUpdateResult)

    def entity_query(
        self, namespace: str, entity_type: Optional[str] = None, query: Optional[str] = None, limit: int = 50
    ) -> EntityQueryResult:
        """Query entities."""
        args: dict[str, Any] = {"namespace": namespace, "limit": limit}
        if entity_type:
            args["entity_type"] = entity_type
        if query:
            args["query"] = query
        return self._tool_call("entity_query", args, EntityQueryResult)

    async def aentity_query(
        self, namespace: str, entity_type: Optional[str] = None, query: Optional[str] = None, limit: int = 50
    ) -> EntityQueryResult:
        """Async query entities."""
        args: dict[str, Any] = {"namespace": namespace, "limit": limit}
        if entity_type:
            args["entity_type"] = entity_type
        if query:
            args["query"] = query
        return await self._atool_call("entity_query", args, EntityQueryResult)

    def relationship_create(
        self, namespace: str, from_uid: str, to_uid: str, relationship_type: RelationshipType
    ) -> RelationshipCreateResult:
        """Create a relationship."""
        return self._tool_call(
            "relationship_create",
            {
                "namespace": namespace,
                "from_uid": from_uid,
                "to_uid": to_uid,
                "relationship_type": relationship_type.value,
            },
            RelationshipCreateResult,
        )

    async arelationship_create(
        self, namespace: str, from_uid: str, to_uid: str, relationship_type: RelationshipType
    ) -> RelationshipCreateResult:
        """Async create a relationship."""
        return await self._atool_call(
            "relationship_create",
            {
                "namespace": namespace,
                "from_uid": from_uid,
                "to_uid": to_uid,
                "relationship_type": relationship_type.value,
            },
            RelationshipCreateResult,
        )

    # ========== DOCUMENT TOOLS ==========

    def document_ingest(
        self, namespace: str, content: str, filename: str, document_type: str = "text"
    ) -> DocumentIngestResult:
        """Ingest a document."""
        return self._tool_call(
            "document_ingest",
            {
                "namespace": namespace,
                "content": content,
                "filename": filename,
                "document_type": document_type,
            },
            DocumentIngestResult,
        )

    async def adocument_ingest(
        self, namespace: str, content: str, filename: str, document_type: str = "text"
    ) -> DocumentIngestResult:
        """Async ingest a document."""
        return await self._atool_call(
            "document_ingest",
            {
                "namespace": namespace,
                "content": content,
                "filename": filename,
                "document_type": document_type,
            },
            DocumentIngestResult,
        )

    def document_list(self, namespace: str, limit: int = 20) -> DocumentListResult:
        """List documents."""
        return self._tool_call("document_list", {"namespace": namespace, "limit": limit}, DocumentListResult)

    async def adocument_list(self, namespace: str, limit: int = 20) -> DocumentListResult:
        """Async list documents."""
        return await self._atool_call(
            "document_list", {"namespace": namespace, "limit": limit}, DocumentListResult
        )

    def document_delete(self, namespace: str, document_id: str) -> None:
        """Delete a document."""
        self._tool_call("document_delete", {"namespace": namespace, "document_id": document_id}, None)

    async def adocument_delete(self, namespace: str, document_id: str) -> None:
        """Async delete a document."""
        await self._atool_call("document_delete", {"namespace": namespace, "document_id": document_id}, None)

    # ========== GROUP TOOLS ==========

    def group_create(self, name: str, description: Optional[str] = None) -> GroupCreateResult:
        """Create a group."""
        args: dict[str, Any] = {"name": name}
        if description:
            args["description"] = description
        return self._tool_call("group_create", args, GroupCreateResult)

    async def agroup_create(self, name: str, description: Optional[str] = None) -> GroupCreateResult:
        """Async create a group."""
        args: dict[str, Any] = {"name": name}
        if description:
            args["description"] = description
        return await self._atool_call("group_create", args, GroupCreateResult)

    def group_list(self) -> GroupListResult:
        """List groups."""
        return self._tool_call("group_list", {}, GroupListResult)

    async def agroup_list(self) -> GroupListResult:
        """Async list groups."""
        return await self._atool_call("group_list", {}, GroupListResult)

    def group_invite(self, group_id: str, username: str, role: str = "subuser") -> GroupInviteResult:
        """Invite a user to a group."""
        return self._tool_call(
            "group_invite", {"group_id": group_id, "username": username, "role": role}, GroupInviteResult
        )

    async def agroup_invite(self, group_id: str, username: str, role: str = "subuser") -> GroupInviteResult:
        """Async invite a user to a group."""
        return await self._atool_call(
            "group_invite", {"group_id": group_id, "username": username, "role": role}, GroupInviteResult
        )

    def group_members(self, group_id: str) -> GroupMembersResult:
        """List group members."""
        return self._tool_call("group_members", {"group_id": group_id}, GroupMembersResult)

    async def agroup_members(self, group_id: str) -> GroupMembersResult:
        """Async list group members."""
        return await self._atool_call("group_members", {"group_id": group_id}, GroupMembersResult)

    def group_share_link(
        self, group_id: str, max_uses: int = 1, expires_in_hours: int = 24
    ) -> GroupShareLinkResult:
        """Create a share link."""
        return self._tool_call(
            "group_share_link",
            {"group_id": group_id, "max_uses": max_uses, "expires_in_hours": expires_in_hours},
            GroupShareLinkResult,
        )

    async def agroup_share_link(
        self, group_id: str, max_uses: int = 1, expires_in_hours: int = 24
    ) -> GroupShareLinkResult:
        """Async create a share link."""
        return await self._atool_call(
            "group_share_link",
            {"group_id": group_id, "max_uses": max_uses, "expires_in_hours": expires_in_hours},
            GroupShareLinkResult,
        )

    # ========== ADMIN TOOLS ==========

    def admin_users_list(self) -> AdminUsersListResult:
        """List all users (admin only)."""
        return self._tool_call("admin_users_list", {}, AdminUsersListResult)

    async def aadmin_users_list(self) -> AdminUsersListResult:
        """Async list all users (admin only)."""
        return await self._atool_call("admin_users_list", {}, AdminUsersListResult)

    def admin_user_update(
        self, username: str, action: AdminAction, role: Optional[UserRole] = None
    ) -> AdminUserUpdateResult:
        """Update a user (admin only)."""
        args: dict[str, Any] = {"username": username, "action": action.value}
        if role:
            args["role"] = role.value
        return self._tool_call("admin_user_update", args, AdminUserUpdateResult)

    async def aadmin_user_update(
        self, username: str, action: AdminAction, role: Optional[UserRole] = None
    ) -> AdminUserUpdateResult:
        """Async update a user (admin only)."""
        args: dict[str, Any] = {"username": username, "action": action.value}
        if role:
            args["role"] = role.value
        return await self._atool_call("admin_user_update", args, AdminUserUpdateResult)

    def admin_metrics(self) -> AdminMetricsResult:
        """Get system metrics (admin only)."""
        return self._tool_call("admin_metrics", {}, AdminMetricsResult)

    async def aadmin_metrics(self) -> AdminMetricsResult:
        """Async get system metrics (admin only)."""
        return await self._atool_call("admin_metrics", {}, AdminMetricsResult)

    def admin_policies_list(self) -> AdminPoliciesListResult:
        """List policies (admin only)."""
        return self._tool_call("admin_policies_list", {}, AdminPoliciesListResult)

    async def aadmin_policies_list(self) -> AdminPoliciesListResult:
        """Async list policies (admin only)."""
        return await self._atool_call("admin_policies_list", {}, AdminPoliciesListResult)

    def admin_policies_set(
        self,
        id: str,
        effect: PolicyEffect,
        subjects: list[str],
        resources: list[str],
        actions: list[str],
        description: Optional[str] = None,
    ) -> AdminPoliciesSetResult:
        """Create or update a policy (admin only)."""
        args: dict[str, Any] = {
            "id": id,
            "effect": effect.value,
            "subjects": subjects,
            "resources": resources,
            "actions": actions,
        }
        if description:
            args["description"] = description
        return self._tool_call("admin_policies_set", args, AdminPoliciesSetResult)

    async def aadmin_policies_set(
        self,
        id: str,
        effect: PolicyEffect,
        subjects: list[str],
        resources: list[str],
        actions: list[str],
        description: Optional[str] = None,
    ) -> AdminPoliciesSetResult:
        """Async create or update a policy (admin only)."""
        args: dict[str, Any] = {
            "id": id,
            "effect": effect.value,
            "subjects": subjects,
            "resources": resources,
            "actions": actions,
        }
        if description:
            args["description"] = description
        return await self._atool_call("admin_policies_set", args, AdminPoliciesSetResult)

    # ========== MCP PROTOCOL ==========

    def tools_list(self) -> ToolsListResult:
        """List all available MCP tools."""
        response = self.client.get("/api/mcp/tools", headers=self._get_headers())
        data = self._handle_response(response)
        return ToolsListResult(**data)

    async def atools_list(self) -> ToolsListResult:
        """Async list all available MCP tools."""
        response = await self.async_client.get("/api/mcp/tools", headers=self._get_headers())
        data = await self._ahandle_response(response)
        return ToolsListResult(**data)

    def _tool_call(self, name: str, arguments: dict[str, Any], result_class: Any) -> Any:
        """Call an MCP tool."""
        response = self.client.post(
            "/api/mcp/tools/call",
            json={"name": name, "arguments": arguments},
            headers=self._get_headers(),
        )
        data = self._handle_response(response)

        if data.get("isError"):
            raise RMKError(f"Tool {name} failed", details=data)

        # Extract content from response
        content_list = data.get("content", [])
        if content_list and content_list[0].get("type") == "text":
            text = content_list[0]["text"]
            parsed = json.loads(text)
            if result_class:
                return result_class(**parsed)
            return parsed
        return None

    async def _atool_call(self, name: str, arguments: dict[str, Any], result_class: Any) -> Any:
        """Async call an MCP tool."""
        response = await self.async_client.post(
            "/api/mcp/tools/call",
            json={"name": name, "arguments": arguments},
            headers=self._get_headers(),
        )
        data = await self._ahandle_response(response)

        if data.get("isError"):
            raise RMKError(f"Tool {name} failed", details=data)

        # Extract content from response
        content_list = data.get("content", [])
        if content_list and content_list[0].get("type") == "text":
            text = content_list[0]["text"]
            parsed = json.loads(text)
            if result_class:
                return result_class(**parsed)
            return parsed
        return None

    def __enter__(self) -> "RMKClient":
        return self

    def __exit__(self, *args: Any) -> None:
        self.close()

    async def __aenter__(self) -> "RMKClient":
        return self

    async def __aexit__(self, *args: Any) -> None:
        await self.aclose()


# ========== TYPES ==========


class NodeType:
    """Node types for memory storage."""

    ENTITY = "Entity"
    FACT = "Fact"
    EVENT = "Event"
    INSIGHT = "Insight"
    PATTERN = "Pattern"

    def __init__(self, value: str):
        self.value = value


class RelationshipType:
    """Relationship types between entities."""

    KNOWS = "KNOWS"
    LIKES = "LIKES"
    WORKS_AT = "WORKS_AT"
    WORKS_ON = "WORKS_ON"
    FRIEND_OF = "FRIEND_OF"
    RELATED_TO = "RELATED_TO"
    PART_OF = "PART_OF"

    def __init__(self, value: str):
        self.value = value


class UserRole:
    """User roles."""

    USER = "user"
    ADMIN = "admin"

    def __init__(self, value: str):
        self.value = value


class AdminAction:
    """Admin actions."""

    PROMOTE = "promote"
    DEMOTE = "demote"
    DELETE = "delete"

    def __init__(self, value: str):
        self.value = value


class PolicyEffect:
    """Policy effects."""

    ALLOW = "ALLOW"
    DENY = "DENY"

    def __init__(self, value: str):
        self.value = value


@dataclass
class AuthResponse:
    """Authentication response."""

    token: str
    username: str
    role: str
    groups: Optional[list[str]] = None


@dataclass
class MemoryNode:
    """Memory node."""

    uid: str
    name: str
    description: str
    node_type: Optional[str] = None
    activation: Optional[float] = None
    tags: Optional[list[str]] = None


@dataclass
class MemoryStoreResult:
    """Memory store result."""

    uid: str
    node_type: str
    namespace: str
    name: Optional[str] = None


@dataclass
class MemorySearchResult:
    """Memory search result."""

    results: list[dict[str, Any]]
    count: int


@dataclass
class MemoryListResult:
    """Memory list result."""

    results: list[Any]
    total: int
    offset: int
    limit: int


@dataclass
class ChatConsultResult:
    """Chat consult result."""

    response: str
    conversation_id: str
    namespace: str


@dataclass
class Conversation:
    """Conversation."""

    id: str
    name: Optional[str] = None
    description: Optional[str] = None
    created_at: Optional[str] = None


@dataclass
class ConversationsListResult:
    """Conversations list result."""

    conversations: list[dict[str, Any]]
    count: int


@dataclass
class Relationship:
    """Relationship between entities."""

    type: str
    target: str

    def to_dict(self) -> dict[str, str]:
        return {"type": self.type, "target": self.target}


@dataclass
class EntityCreateResult:
    """Entity create result."""

    uid: str
    name: str
    type: str


@dataclass
class EntityUpdateResult:
    """Entity update result."""

    uid: str
    status: str


@dataclass
class EntityQueryResult:
    """Entity query result."""

    entities: list[dict[str, Any]]
    count: int


@dataclass
class RelationshipCreateResult:
    """Relationship create result."""

    status: str
    from_uid: str
    to_uid: str
    rel_type: str


@dataclass
class DocumentIngestResult:
    """Document ingest result."""

    status: str
    filename: str
    document_id: str
    entities_extracted: int


@dataclass
class DocumentListResult:
    """Document list result."""

    documents: list[dict[str, Any]]
    count: int


@dataclass
class GroupCreateResult:
    """Group create result."""

    group_id: str
    namespace: str
    name: str


@dataclass
class GroupListResult:
    """Group list result."""

    groups: list[dict[str, Any]]
    count: int


@dataclass
class GroupInviteResult:
    """Group invite result."""

    invitation_id: str
    status: str


@dataclass
class GroupMembersResult:
    """Group members result."""

    group_id: str
    members: list[dict[str, Any]]
    count: int


@dataclass
class GroupShareLinkResult:
    """Group share link result."""

    token: str
    url: str


@dataclass
class AdminUsersListResult:
    """Admin users list result."""

    users: list[dict[str, Any]]
    count: int


@dataclass
class AdminUserUpdateResult:
    """Admin user update result."""

    status: str
    username: str
    role: Optional[str] = None


@dataclass
class AdminMetricsResult:
    """Admin metrics result."""

    total_users: Optional[int] = None
    total_groups: Optional[int] = None
    total_nodes: Optional[int] = None
    total_conversations: Optional[int] = None
    uptime_seconds: Optional[float] = None


@dataclass
class Policy:
    """Policy."""

    id: str
    effect: str
    subjects: list[str]
    resources: list[str]
    actions: list[str]
    description: Optional[str] = None


@dataclass
class AdminPoliciesListResult:
    """Admin policies list result."""

    policies: list[dict[str, Any]]
    count: int


@dataclass
class AdminPoliciesSetResult:
    """Admin policies set result."""

    status: str
    id: str
    effect: str


@dataclass
class MCPTool:
    """MCP tool definition."""

    name: str
    description: str
    input_schema: dict[str, Any]

    def __init__(self, **kwargs: Any):
        self.name = kwargs.get("name", "")
        self.description = kwargs.get("description", "")
        self.input_schema = kwargs.get("inputSchema", kwargs.get("input_schema", {}))


@dataclass
class ToolsListResult:
    """Tools list result."""

    tools: list[dict[str, Any]]

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "ToolsListResult":
        return cls(tools=[MCPTool(**t).__dict__ for t in data.get("tools", [])])


__all__ = [
    "RMKClient",
    "RMKError",
    "NodeType",
    "RelationshipType",
    "UserRole",
    "AdminAction",
    "PolicyEffect",
    "AuthResponse",
    "MemoryNode",
    "MemoryStoreResult",
    "MemorySearchResult",
    "MemoryListResult",
    "ChatConsultResult",
    "Conversation",
    "ConversationsListResult",
    "Relationship",
    "EntityCreateResult",
    "EntityUpdateResult",
    "EntityQueryResult",
    "RelationshipCreateResult",
    "DocumentIngestResult",
    "DocumentListResult",
    "GroupCreateResult",
    "GroupListResult",
    "GroupInviteResult",
    "GroupMembersResult",
    "GroupShareLinkResult",
    "AdminUsersListResult",
    "AdminUserUpdateResult",
    "AdminMetricsResult",
    "Policy",
    "AdminPoliciesListResult",
    "AdminPoliciesSetResult",
    "MCPTool",
    "ToolsListResult",
]
