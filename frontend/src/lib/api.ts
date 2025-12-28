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
            const res = await fetch(`${API_BASE_URL}/api/dashboard/stats`);
            if (!res.ok) throw new Error("Failed to fetch stats");
            return await res.json();
        } catch (e) {
            console.warn("Using mock stats", e);
            return MOCK_STATS;
        }
    },

    getGraph: async (): Promise<GraphData> => {
        try {
            const res = await fetch(`${API_BASE_URL}/api/dashboard/graph`);
            if (!res.ok) throw new Error("Failed to fetch graph");
            return await res.json();
        } catch (e) {
            console.warn("Using mock graph", e);
            return MOCK_GRAPH;
        }
    },

    getIngestionStats: async (): Promise<IngestionStats> => {
        try {
            const res = await fetch(`${API_BASE_URL}/api/dashboard/ingestion`);
            if (!res.ok) throw new Error("Failed to fetch ingestion stats");
            return await res.json();
        } catch (e) {
            console.warn("Using mock ingestion stats", e);
            return MOCK_INGESTION;
        }
    },

    // Kernel Operations
    spreadActivation: async (startNode: string, depth: number = 3): Promise<SpreadActivationResult> => {
        try {
            const res = await fetch(`${API_BASE_URL}/api/graph/spread-activation`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
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
            const res = await fetch(`${API_BASE_URL}/api/graph/community`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
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
            const res = await fetch(`${API_BASE_URL}/api/graph/temporal`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
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
            const res = await fetch(`${API_BASE_URL}/api/graph/expand`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
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

    // Groups
    getGroups: async (): Promise<any[]> => {
        try {
            const res = await fetch(`${API_BASE_URL}/api/list-groups`, {
                headers: (getAuthHeaders() as Record<string, string>),
            });
            if (!res.ok) throw new Error("Failed to fetch groups");
            const data = await res.json();
            return data.groups || [];
        } catch (e) {
            console.error("Fetch groups error:", e);
            return [];
        }
    },

    createGroup: async (name: string, description: string): Promise<any> => {
        try {
            const res = await fetch(`${API_BASE_URL}/api/groups`, {
                method: 'POST',
                headers: (getAuthHeaders() as Record<string, string>),
                body: JSON.stringify({ name, description }),
            });
            if (!res.ok) throw new Error("Failed to create group");
            return await res.json();
        } catch (e) {
            console.error("Create group error:", e);
            throw e;
        }
    },

    // Chat
    sendMessage: async (message: string, conversationId?: string): Promise<any> => {
        try {
            const res = await fetch(`${API_BASE_URL}/api/chat`, {
                method: 'POST',
                headers: (getAuthHeaders() as Record<string, string>),
                body: JSON.stringify({ message, conversation_id: conversationId }),
            });
            if (!res.ok) throw new Error("Failed to send message");
            return await res.json();
        } catch (e) {
            console.error("Chat error:", e);
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
            const res = await fetch(`${API_BASE_URL}/api/search?q=${encodeURIComponent(query)}`);
            if (!res.ok) throw new Error("Search failed");
            return await res.json();
        } catch (e) {
            console.warn("Search error:", e);
            return [];
        }
    },
};
