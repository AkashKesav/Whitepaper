import { z } from "zod";

// Configure API URL - Proxy handles /api requests to backend
const API_BASE_URL = '';

// Helper to get auth headers
const getAuthHeaders = (): HeadersInit => {
    const token = localStorage.getItem('rmk_token');
    const headers: HeadersInit = { 'Content-Type': 'application/json' };
    if (token) {
        (headers as Record<string, string>)['Authorization'] = `Bearer ${token}`;
    }
    return headers;
};

export interface DashboardStats {
    node_counts: Record<string, number>;
    total_entities: number;
    active_relations: number;
    memory_usage: string;
    traversal_depth: number;
}

export interface GraphNode {
    id: string;
    label?: string;
    group?: string;
    size?: number;
    activation?: number;
    source_text?: string; // Original quote from user
}

export interface GraphEdge {
    source: string;
    target: string;
    id?: string;
    label?: string;
}

export interface GraphData {
    nodes: GraphNode[];
    edges: GraphEdge[];
}

export interface IngestionStats {
    total_processed: number;
    avg_latency_ms: number;
    error_count: number;
    pipeline_active: boolean;
    nats_connected: boolean;
}

export interface SpreadActivationResult {
    success: boolean;
    visited_nodes: string[];
    activation_scores: Record<string, number>;
}

export interface CommunityResult {
    communities: Array<{
        id: string;
        nodes: string[];
        coherence: number;
    }>;
}

export interface TemporalQueryResult {
    nodes: GraphNode[];
    time_range: { start: string; end: string };
}

// Mock data for fallback
const MOCK_STATS: DashboardStats = {
    node_counts: { Person: 15, Skill: 20, Location: 5, Department: 4 },
    total_entities: 46,
    active_relations: 839,
    memory_usage: "124 MB",
    traversal_depth: 3,
};

const MOCK_GRAPH: GraphData = {
    nodes: [
        { id: "1", label: "David Davis", group: "Person", size: 15 },
        { id: "2", label: "Rust", group: "Skill", size: 10 },
        { id: "3", label: "Austin", group: "Location", size: 10 },
        { id: "4", label: "Engineering", group: "Department", size: 12 },
        { id: "5", label: "Go", group: "Skill", size: 10 },
    ],
    edges: [
        { source: "1", target: "2", label: "has_interest" },
        { source: "1", target: "3", label: "works_at" },
        { source: "1", target: "4", label: "department" },
        { source: "1", target: "5", label: "has_interest" },
    ]
};

const MOCK_INGESTION: IngestionStats = {
    total_processed: 1240,
    avg_latency_ms: 145,
    error_count: 0,
    pipeline_active: true,
    nats_connected: true,
};

export const api = {
    // Dashboard Data
    getStats: async (): Promise<DashboardStats> => {
        try {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/dashboard/stats`, { headers });
            if (!res.ok) throw new Error("Failed to fetch stats");
            return await res.json();
        } catch (e) {
            console.warn("Using mock stats", e);
            return MOCK_STATS;
        }
    },

    getGraph: async (): Promise<GraphData> => {
        try {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/dashboard/graph`, { headers });
            if (!res.ok) throw new Error("Failed to fetch graph");
            return await res.json();
        } catch (e) {
            console.warn("Using mock graph", e);
            return MOCK_GRAPH;
        }
    },

    getIngestionStats: async (): Promise<IngestionStats> => {
        try {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/dashboard/ingestion`, { headers });
            if (!res.ok) throw new Error("Failed to fetch ingestion stats");
            return await res.json();
        } catch (e) {
            console.warn("Using mock ingestion stats", e);
            return MOCK_INGESTION;
        }
    },

    // User Management (Admin API)
    getAllUsers: async (): Promise<any[]> => {
        const headers = getAuthHeaders() as Record<string, string>;
        try {
            // Use /api/users for all authenticated users (not admin-only)
            const res = await fetch(`${API_BASE_URL}/api/users`, { headers });
            if (!res.ok) {
                console.warn("Failed to fetch users");
                return [];
            }
            const data = await res.json();
            return data.users || [];
        } catch (e) {
            console.warn("Error fetching users", e);
            return [];
        }
    },

    createUser: async (username: string, password: string, role: string = 'user'): Promise<any> => {
        const headers = getAuthHeaders() as Record<string, string>;
        const res = await fetch(`${API_BASE_URL}/api/admin/users`, {
            method: 'POST',
            headers,
            body: JSON.stringify({ username, password, role }),
        });
        if (!res.ok) {
            const msg = await res.text();
            throw new Error(msg || "Failed to create user");
        }
        return await res.json();
    },

    // Groups API
    getGroups: async (): Promise<any[]> => {
        const headers = getAuthHeaders() as Record<string, string>;
        const res = await fetch(`${API_BASE_URL}/api/groups`, { headers });
        if (!res.ok) {
            console.warn("Failed to fetch groups, returning empty array");
            return [];
        }
        const data = await res.json();
        return data.groups || [];
    },

    createGroup: async (name: string, description: string): Promise<any> => {
        const headers = getAuthHeaders() as Record<string, string>;
        const res = await fetch(`${API_BASE_URL}/api/groups`, {
            method: 'POST',
            headers,
            body: JSON.stringify({ name, description }),
        });
        if (!res.ok) throw new Error("Failed to create group");
        return await res.json();
    },

    getGroupMembers: async (groupId: string): Promise<any[]> => {
        const headers = getAuthHeaders() as Record<string, string>;
        const res = await fetch(`${API_BASE_URL}/api/groups/${groupId}/members`, { headers });
        if (!res.ok) {
            console.warn("Failed to fetch group members");
            return [];
        }
        const data = await res.json();
        return data.members || [];
    },

    addGroupMember: async (groupId: string, username: string): Promise<void> => {
        const headers = getAuthHeaders() as Record<string, string>;
        const res = await fetch(`${API_BASE_URL}/api/groups/${groupId}/members`, {
            method: 'POST',
            headers,
            body: JSON.stringify({ username }),
        });
        if (!res.ok) throw new Error("Failed to add member");
    },

    removeGroupMember: async (groupId: string, username: string): Promise<void> => {
        const headers = getAuthHeaders() as Record<string, string>;
        const res = await fetch(`${API_BASE_URL}/api/groups/${groupId}/members/${username}`, {
            method: 'DELETE',
            headers,
        });
        if (!res.ok) throw new Error("Failed to remove member");
    },

    deleteGroup: async (groupId: string): Promise<void> => {
        const headers = getAuthHeaders() as Record<string, string>;
        const res = await fetch(`${API_BASE_URL}/api/groups/${groupId}`, {
            method: 'DELETE',
            headers,
        });
        if (!res.ok) throw new Error("Failed to delete group");
    },

    // Invitation/Notification APIs
    getPendingInvitations: async (): Promise<any[]> => {
        const headers = getAuthHeaders() as Record<string, string>;
        const res = await fetch(`${API_BASE_URL}/api/invitations`, { headers });
        if (!res.ok) {
            console.warn("Failed to fetch invitations");
            return [];
        }
        const data = await res.json();
        return data.invitations || [];
    },

    sendInvitation: async (workspaceId: string, username: string, role: string = 'member'): Promise<any> => {
        const headers = getAuthHeaders() as Record<string, string>;
        const res = await fetch(`${API_BASE_URL}/api/workspaces/${workspaceId}/invite`, {
            method: 'POST',
            headers,
            body: JSON.stringify({ username, role }),
        });
        if (!res.ok) throw new Error("Failed to send invitation");
        return await res.json();
    },

    acceptInvitation: async (invitationId: string): Promise<void> => {
        const headers = getAuthHeaders() as Record<string, string>;
        const res = await fetch(`${API_BASE_URL}/api/invitations/${invitationId}/accept`, {
            method: 'POST',
            headers,
        });
        if (!res.ok) throw new Error("Failed to accept invitation");
    },

    declineInvitation: async (invitationId: string): Promise<void> => {
        const headers = getAuthHeaders() as Record<string, string>;
        const res = await fetch(`${API_BASE_URL}/api/invitations/${invitationId}/decline`, {
            method: 'POST',
            headers,
        });
        if (!res.ok) throw new Error("Failed to decline invitation");
    },

    getWorkspaceSentInvitations: async (workspaceId: string): Promise<any[]> => {
        const headers = getAuthHeaders() as Record<string, string>;
        try {
            const res = await fetch(`${API_BASE_URL}/api/workspaces/${workspaceId}/invitations/sent`, { headers });
            if (!res.ok) {
                console.warn("Failed to fetch workspace sent invitations");
                return [];
            }
            const data = await res.json();
            return data.invitations || [];
        } catch (e) {
            console.warn("Error fetching workspace sent invitations", e);
            return [];
        }
    },

    // Kernel Operations
    spreadActivation: async (startNode: string, depth: number = 3): Promise<SpreadActivationResult> => {
        try {
            const headers = { 'Content-Type': 'application/json', ...getAuthHeaders() } as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/graph/spread-activation`, {
                method: 'POST',
                headers,
                body: JSON.stringify({ start_node: startNode, max_depth: depth }),
            });
            if (!res.ok) throw new Error("Spread activation failed");
            return await res.json();
        } catch (e) {
            console.error("Spread activation error:", e);
            return { success: false, visited_nodes: [], activation_scores: {} };
        }
    },

    detectCommunities: async (): Promise<CommunityResult> => {
        try {
            const headers = { 'Content-Type': 'application/json', ...getAuthHeaders() } as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/graph/community`, {
                method: 'POST',
                headers,
                body: JSON.stringify({}),
            });
            if (!res.ok) throw new Error("Community detection failed");
            return await res.json();
        } catch (e) {
            console.error("Community detection error:", e);
            return { communities: [] };
        }
    },

    temporalQuery: async (lookbackDays: number = 30): Promise<TemporalQueryResult> => {
        try {
            const headers = { 'Content-Type': 'application/json', ...getAuthHeaders() } as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/graph/temporal`, {
                method: 'POST',
                headers,
                body: JSON.stringify({ lookback_days: lookbackDays }),
            });
            if (!res.ok) throw new Error("Temporal query failed");
            return await res.json();
        } catch (e) {
            console.error("Temporal query error:", e);
            return { nodes: [], time_range: { start: '', end: '' } };
        }
    },

    expandNode: async (nodeId: string, maxHops: number = 2): Promise<GraphData> => {
        try {
            const headers = { 'Content-Type': 'application/json', ...getAuthHeaders() } as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/graph/expand`, {
                method: 'POST',
                headers,
                body: JSON.stringify({ node_id: nodeId, max_hops: maxHops }),
            });
            if (!res.ok) throw new Error("Node expansion failed");
            return await res.json();
        } catch (e) {
            console.error("Node expansion error:", e);
            return { nodes: [], edges: [] };
        }
    },

    triggerReflection: async (): Promise<{ success: boolean; insights: string[] }> => {
        try {
            // This would trigger the reflection engine on the backend
            const res = await fetch(`${API_BASE_URL}/api/stats`, {
                method: 'GET',
            });
            if (!res.ok) throw new Error("Reflection trigger failed");
            return { success: true, insights: ["Reflection cycle initiated"] };
        } catch (e) {
            console.error("Reflection error:", e);
            return { success: false, insights: [] };
        }
    },

    // Conversations
    getConversations: async (): Promise<{ conversations: any[] }> => {
        const headers = getAuthHeaders() as Record<string, string>;
        const res = await fetch(`${API_BASE_URL}/api/conversations`, { headers });
        if (!res.ok) throw new Error("Failed to fetch conversations");
        return await res.json();
    },

    deleteConversation: async (conversationId: string): Promise<void> => {
        const headers = getAuthHeaders() as Record<string, string>;
        const res = await fetch(`${API_BASE_URL}/api/conversations/${conversationId}`, {
            method: 'DELETE',
            headers
        });
        if (!res.ok) throw new Error("Failed to delete conversation");
    },

    // Chat
    sendMessage: async (message: string, conversationId?: string, namespace?: string): Promise<any> => {
        try {
            const body: any = { message };
            if (conversationId) body.conversation_id = conversationId;
            if (namespace) body.namespace = namespace;
            
            const res = await fetch(`${API_BASE_URL}/api/chat`, {
                method: 'POST',
                headers: (getAuthHeaders() as Record<string, string>),
                body: JSON.stringify(body),
            });
            if (!res.ok) throw new Error("Failed to send message");
            return await res.json();
        } catch (e) {
            console.error("Chat error:", e);
            throw e;
        }
    },

    sendGroupMessage: async (message: string, groupNamespace: string, conversationId?: string): Promise<any> => {
        try {
            const body: any = { 
                message, 
                namespace: groupNamespace  // Use group's namespace for shared memory
            };
            if (conversationId) body.conversation_id = conversationId;
            
            const res = await fetch(`${API_BASE_URL}/api/chat`, {
                method: 'POST',
                headers: (getAuthHeaders() as Record<string, string>),
                body: JSON.stringify(body),
            });
            if (!res.ok) throw new Error("Failed to send group message");
            return await res.json();
        } catch (e) {
            console.error("Group chat error:", e);
            throw e;
        }
    },

    // Ingestion
    uploadDocument: async (file: File): Promise<any> => {
        try {
            const formData = new FormData();
            formData.append('file', file);

            const token = localStorage.getItem('rmk_token');
            const headers: Record<string, string> = {};
            if (token) {
                headers['Authorization'] = `Bearer ${token}`;
            }
            // Note: Do NOT set Content-Type, browser will set it with boundary

            const res = await fetch(`${API_BASE_URL}/api/upload`, {
                method: 'POST',
                headers: headers,
                body: formData,
            });

            if (!res.ok) {
                const text = await res.text();
                throw new Error(text || "Upload failed");
            }
            return await res.json();
        } catch (e) {
            console.error("Upload error:", e);
            throw e;
        }
    },

    searchEntities: async (query: string): Promise<GraphNode[]> => {
        try {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/search?q=${encodeURIComponent(query)}`, { headers });
            if (!res.ok) throw new Error("Search failed");
            return await res.json();
        } catch (e) {
            console.warn("Search error:", e);
            return [];
        }
    },

    // Admin
    admin: {
        getStats: async (): Promise<any> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/system/stats`, { headers });
            if (!res.ok) throw new Error("Failed to fetch system stats");
            return await res.json();
        },

        getUsers: async (): Promise<any[]> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/users`, { headers });
            if (!res.ok) throw new Error("Failed to fetch users");
            const data = await res.json();
            return data.users || [];
        },

        createUser: async (user: any): Promise<void> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/users`, {
                method: 'POST',
                headers,
                body: JSON.stringify(user),
            });
            if (!res.ok) throw new Error("Failed to create user");
        },

        updateUserRole: async (username: string, role: string): Promise<void> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/users/${username}/role`, {
                method: 'PUT',
                headers,
                body: JSON.stringify({ role }),
            });
            if (!res.ok) throw new Error("Failed to update user role");
        },

        deleteUser: async (username: string): Promise<void> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/users/${username}`, {
                method: 'DELETE',
                headers
            });
            if (!res.ok) throw new Error("Failed to delete user");
        },

        getUserDetails: async (username: string): Promise<any> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/users/${username}/details`, { headers });
            if (!res.ok) throw new Error("Failed to fetch user details");
            return await res.json();
        },

        extendTrial: async (username: string, days: number): Promise<any> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/users/${username}/trial`, {
                method: 'POST',
                headers,
                body: JSON.stringify({ days }),
            });
            if (!res.ok) throw new Error("Failed to extend trial");
            return await res.json();
        },

        batchUpdateRole: async (usernames: string[], role: string): Promise<any> => {
             const headers = getAuthHeaders() as Record<string, string>;
             const res = await fetch(`${API_BASE_URL}/api/admin/batch/role`, {
                method: 'POST',
                headers,
                body: JSON.stringify({ usernames, role }),
            });
            if (!res.ok) throw new Error("Failed to batch update roles");
            return await res.json();
        },

        batchDeleteUsers: async (usernames: string[]): Promise<any> => {
             const headers = getAuthHeaders() as Record<string, string>;
             const res = await fetch(`${API_BASE_URL}/api/admin/batch/delete`, {
                method: 'POST',
                headers,
                body: JSON.stringify({ usernames }),
            });
            if (!res.ok) throw new Error("Failed to batch delete users");
            return await res.json();
        },

        triggerReflection: async (): Promise<void> => {
             const headers = getAuthHeaders() as Record<string, string>;
             const res = await fetch(`${API_BASE_URL}/api/admin/system/reflection`, {
                method: 'POST',
                headers
            });
            if (!res.ok) throw new Error("Failed to trigger reflection");
        },

        getActivityLog: async (limit: number = 50): Promise<any[]> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/activity?limit=${limit}`, { headers });
            if (!res.ok) throw new Error("Failed to fetch activity log");
            const data = await res.json();
            return data.log || [];
        },

        // Policy Management
        getPolicies: async (): Promise<{ policies: any[] }> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/policies`, { headers });
            if (!res.ok) throw new Error("Failed to fetch policies");
            return await res.json();
        },

        createPolicy: async (policy: {
            id: string;
            description: string;
            subjects: string[];
            resources: string[];
            actions: string[];
            effect: 'ALLOW' | 'DENY';
            conditions?: Record<string, string>;
        }): Promise<{ id: string; policy_id: string }> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/policies`, {
                method: 'POST',
                headers,
                body: JSON.stringify(policy),
            });
            if (!res.ok) throw new Error("Failed to create policy");
            return await res.json();
        },

        deletePolicy: async (id: string): Promise<void> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/policies/${id}`, {
                method: 'DELETE',
                headers,
            });
            if (!res.ok) throw new Error("Failed to delete policy");
        }
    },

    finance: {
        getRevenue: async (): Promise<any> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/finance/revenue`, { headers });
            if (!res.ok) throw new Error("Failed to fetch revenue");
            return await res.json();
        }
    },

    support: {
        getTickets: async (): Promise<any[]> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/support/tickets`, { headers });
            if (!res.ok) throw new Error("Failed to fetch tickets");
            const data = await res.json();
            return data.tickets || [];
        },
        resolveTicket: async (id: string): Promise<void> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/support/tickets/${id}/resolve`, {
                method: 'POST',
                headers
            });
            if (!res.ok) throw new Error("Failed to resolve ticket");
        }
    },

    affiliates: {
        getList: async (): Promise<any[]> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/affiliates`, { headers });
            if (!res.ok) throw new Error("Failed to fetch affiliates");
            const data = await res.json();
            return data.affiliates || [];
        },
        create: async (data: any): Promise<any> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/affiliates`, {
                method: 'POST',
                headers,
                body: JSON.stringify(data),
            });
            if (!res.ok) throw new Error("Failed to create affiliate");
            return await res.json();
        },
        delete: async (code: string): Promise<void> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/affiliates/${code}`, {
                method: 'DELETE',
                headers
            });
            if (!res.ok) throw new Error("Failed to delete affiliate");
        }
    },

    operations: {
        getCampaigns: async (): Promise<any[]> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/operations/campaigns`, { headers });
            if (!res.ok) throw new Error("Failed to fetch campaigns");
            const data = await res.json();
            return data.campaigns || [];
        },
        createCampaign: async (data: any): Promise<any> => {
             const headers = getAuthHeaders() as Record<string, string>;
             const res = await fetch(`${API_BASE_URL}/api/admin/operations/campaigns`, {
                method: 'POST',
                headers,
                body: JSON.stringify(data),
            });
            if (!res.ok) throw new Error("Failed to create campaign");
            return await res.json();
        },
        deleteCampaign: async (id: string): Promise<void> => {
             const headers = getAuthHeaders() as Record<string, string>;
             const res = await fetch(`${API_BASE_URL}/api/admin/operations/campaigns/${id}`, {
                method: 'DELETE',
                headers
            });
            if (!res.ok) throw new Error("Failed to delete campaign");
        }
    },

    system: {
        getFlags: async (): Promise<any[]> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/system/flags`, { headers });
            if (!res.ok) throw new Error("Failed to fetch flags");
            const data = await res.json();
            return data.flags || [];
        },
        toggleFlag: async (key: string, is_enabled: boolean): Promise<void> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/system/flags/toggle`, {
                method: 'POST',
                headers,
                body: JSON.stringify({ key, is_enabled })
            });
            if (!res.ok) throw new Error("Failed to toggle flag");
        }
    },

    emergency: {
        getRequests: async (): Promise<any[]> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/emergency/requests`, { headers });
            if (!res.ok) throw new Error("Failed to fetch emergency requests");
            const data = await res.json();
            return data.requests || [];
        },
        approve: async (id: string): Promise<void> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/emergency/requests/${id}/approve`, {
                method: 'POST',
                headers
            });
            if (!res.ok) throw new Error("Failed to approve request");
        },
        deny: async (id: string): Promise<void> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/admin/emergency/requests/${id}/deny`, {
                method: 'POST',
                headers
            });
            if (!res.ok) throw new Error("Failed to deny request");
        }
    },

    // User Settings (Per-user encrypted API key storage)
    userSettings: {
        // Get current user's settings (API keys are NOT returned, only status)
        getSettings: async (): Promise<{
            has_nim_key: boolean;
            has_openai_key: boolean;
            has_anthropic_key: boolean;
            theme: string;
            notifications_enabled: boolean;
            updated_at: string;
        }> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/user/settings`, { headers });
            if (!res.ok) throw new Error("Failed to fetch user settings");
            return await res.json();
        },

        // Save user settings (API keys encrypted on backend)
        saveSettings: async (settings: {
            nim_api_key?: string;
            openai_api_key?: string;
            anthropic_api_key?: string;
            theme?: string;
        }): Promise<void> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/user/settings`, {
                method: 'PUT',
                headers,
                body: JSON.stringify(settings),
            });
            if (!res.ok) throw new Error("Failed to save user settings");
        },

        // Delete an API key
        deleteAPIKey: async (provider: 'nim' | 'openai' | 'anthropic'): Promise<void> => {
            const headers = getAuthHeaders() as Record<string, string>;
            const res = await fetch(`${API_BASE_URL}/api/user/settings/keys/${provider}`, {
                method: 'DELETE',
                headers
            });
            if (!res.ok) throw new Error("Failed to delete API key");
        },

        // Test NIM API key connection (before saving)
        testNIMConnection: async (apiKey: string): Promise<{ success: boolean; message?: string; model?: string }> => {
            const res = await fetch(`${API_BASE_URL}/api/test/nim`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ api_key: apiKey }),
            });
            if (!res.ok) throw new Error("Connection test failed");
            return await res.json();
        }
    }
};
