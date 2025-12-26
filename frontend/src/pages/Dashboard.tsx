import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { api, GraphData } from '@/lib/api';
import { MemoryGraph2D, GraphNode } from '@/components/graph/MemoryGraph2D';
import { NodeDetailsPanel } from '@/components/graph/NodeDetailsPanel';
import { useAuth } from '@/contexts/AuthContext';
import { cn } from '@/lib/utils';
import {
    Lightbulb, TrendingUp, AlertTriangle, Calendar,
    MessageCircle, Users, Settings, Upload, BarChart3,
    Globe, BookOpen, Shield
} from 'lucide-react';

const SidebarItem = ({ icon, label, active = false, onClick }: { icon: React.ReactNode, label: string, active?: boolean, onClick?: () => void }) => (
    <a href="#" onClick={onClick} className={cn(
        "mx-3 px-3 py-2 rounded-lg flex items-center gap-3 transition-colors",
        active
            ? "bg-purple-500/10 text-purple-400 border border-purple-500/20"
            : "text-zinc-400 hover:text-white hover:bg-white/5"
    )}>
        {icon}
        <span className="hidden lg:block font-medium">{label}</span>
    </a>
)

const StatCard = ({ label, value, subtext, color = "text-white" }: { label: string, value: string | number, subtext?: string, color?: string }) => (
    <div className="flex flex-col">
        <span className="text-[10px] text-zinc-500 uppercase tracking-wider">{label}</span>
        <span className={cn("text-sm font-mono", color)}>{value}</span>
    </div>
)

export const Dashboard = () => {
    const queryClient = useQueryClient();
    const navigate = useNavigate();
    const { isAdmin } = useAuth();

    // State
    const [startNode, setStartNode] = useState('');
    const [lookbackDays, setLookbackDays] = useState(30);
    const [operationLog, setOperationLog] = useState<string[]>([]);
    const [searchQuery, setSearchQuery] = useState('');
    const [selectedNode, setSelectedNode] = useState<GraphNode | null>(null);

    // Add log entry
    const log = (message: string) => {
        setOperationLog(prev => [...prev.slice(-9), `[${new Date().toLocaleTimeString()}] ${message}`]);
    };

    // Queries
    const { data: stats, isLoading: statsLoading } = useQuery({
        queryKey: ['dashboard-stats'],
        queryFn: api.getStats,
        refetchInterval: 30000, // Refresh every 30s
    });
    const { data: graphData, isLoading: graphLoading } = useQuery({
        queryKey: ['dashboard-graph'],
        queryFn: api.getGraph,
        refetchInterval: 60000, // Refresh every minute
    });
    const { data: ingestion } = useQuery({
        queryKey: ['dashboard-ingestion'],
        queryFn: api.getIngestionStats,
        refetchInterval: 10000, // Refresh every 10s
    });

    // Mutations for kernel operations
    const spreadActivationMutation = useMutation({
        mutationFn: (node: string) => api.spreadActivation(node, 3),
        onSuccess: (data) => {
            log(`Spread activation complete: ${data.visited_nodes?.length || 0} nodes visited`);
            queryClient.invalidateQueries({ queryKey: ['dashboard-graph'] });
        },
        onError: () => log('Spread activation failed'),
    });

    const communityMutation = useMutation({
        mutationFn: api.detectCommunities,
        onSuccess: (data) => {
            log(`Community detection: ${data.communities?.length || 0} clusters found`);
        },
        onError: () => log('Community detection failed'),
    });

    const temporalMutation = useMutation({
        mutationFn: (days: number) => api.temporalQuery(days),
        onSuccess: (data) => {
            log(`Temporal query: ${data.nodes?.length || 0} nodes in range`);
        },
        onError: () => log('Temporal query failed'),
    });

    const reflectionMutation = useMutation({
        mutationFn: api.triggerReflection,
        onSuccess: (data) => {
            log(`Reflection: ${data.success ? 'triggered' : 'failed'}`);
        },
        onError: () => log('Reflection trigger failed'),
    });

    // Handlers
    const handleSpreadActivation = () => {
        if (!startNode.trim()) {
            log('Error: Enter a start node');
            return;
        }
        log(`Starting spread activation from "${startNode}"...`);
        spreadActivationMutation.mutate(startNode);
    };

    const handleCommunityDetection = () => {
        log('Running community detection...');
        communityMutation.mutate();
    };

    const handleTemporalDecay = () => {
        log(`Applying temporal decay (${lookbackDays} days)...`);
        temporalMutation.mutate(lookbackDays);
    };

    const handleSearch = (e: React.FormEvent) => {
        e.preventDefault();
        if (searchQuery.trim()) {
            log(`Searching for "${searchQuery}"...`);
            // Could trigger a search mutation here
        }
    };

    return (
        <div className="h-screen flex flex-col overflow-hidden bg-[#09090b] text-white">

            {/* Header */}
            <header className="h-16 border-b border-white/10 flex items-center px-6 justify-between bg-zinc-950/80 backdrop-blur z-20 relative">
                <div className="flex items-center gap-3">
                    <div className="w-8 h-8 rounded-lg bg-purple-600 flex items-center justify-center font-bold shadow-[0_0_15px_rgba(147,51,234,0.5)]">MK</div>
                    <span className="font-semibold text-lg tracking-tight">MemoryKernel</span>
                </div>

                <form onSubmit={handleSearch} className="flex-1 max-w-xl mx-8 relative">
                    <svg className="w-4 h-4 text-zinc-500 absolute left-3 top-1/2 -translate-y-1/2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" /></svg>
                    <input
                        type="text"
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        placeholder="Search entities, memories, or patterns..."
                        className="w-full bg-zinc-900/50 border border-white/10 rounded-full py-2 pl-10 pr-4 text-sm text-zinc-200 focus:outline-none focus:border-purple-500/50 focus:ring-1 focus:ring-purple-500/20 transition-all"
                    />
                </form>

                <div className="flex items-center gap-4 text-sm text-zinc-400">
                    <span className="flex items-center gap-2 px-3 py-1 rounded-full bg-green-500/10 text-green-400 border border-green-500/20">
                        <span className="w-1.5 h-1.5 rounded-full bg-green-500 animate-pulse"></span>
                        System Online
                    </span>
                    <div className="w-8 h-8 rounded-full bg-gradient-to-br from-purple-500 to-indigo-600 border border-white/10"></div>
                </div>
            </header>

            <div className="flex-1 flex overflow-hidden relative">

                {/* Sidebar */}
                <aside className="w-16 lg:w-64 border-r border-white/10 bg-zinc-950 flex flex-col pt-6 gap-2 z-10">
                    <div className="px-4 text-xs font-semibold text-zinc-500 mb-2 uppercase tracking-wider hidden lg:block">BrainOS</div>

                    <SidebarItem active icon={<BarChart3 className="w-5 h-5" />} label="Graph View" />
                    <SidebarItem onClick={() => navigate('/chat')} icon={<MessageCircle className="w-5 h-5" />} label="Chat" />
                    <SidebarItem onClick={() => navigate('/ingestion')} icon={<Upload className="w-5 h-5" />} label="Ingestion" />

                    <div className="px-4 text-xs font-semibold text-zinc-500 mb-2 mt-6 uppercase tracking-wider hidden lg:block">Social</div>
                    <SidebarItem onClick={() => navigate('/groups')} icon={<Users className="w-5 h-5" />} label="Groups" />

                    <div className="px-4 text-xs font-semibold text-zinc-500 mb-2 mt-6 uppercase tracking-wider hidden lg:block">Analysis</div>
                    <SidebarItem onClick={handleCommunityDetection} icon={<Globe className="w-5 h-5" />} label="Communities" />
                    <SidebarItem onClick={() => reflectionMutation.mutate()} icon={<BookOpen className="w-5 h-5" />} label="Insights" />

                    <div className="mt-auto" />
                    <div className="px-4 text-xs font-semibold text-zinc-500 mb-2 mt-6 uppercase tracking-wider hidden lg:block">System</div>
                    <SidebarItem onClick={() => navigate('/settings')} icon={<Settings className="w-5 h-5" />} label="Settings" />

                    {/* Admin Section - only visible to admins */}
                    {isAdmin && (
                        <>
                            <div className="px-4 text-xs font-semibold text-zinc-500 mb-2 mt-6 uppercase tracking-wider hidden lg:block">Admin</div>
                            <SidebarItem
                                onClick={() => navigate('/admin')}
                                icon={<svg className="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" /></svg>}
                                label="Admin Panel"
                            />
                        </>
                    )}
                </aside>

                <main className="flex-1 overflow-hidden relative flex flex-col">
                    <div className="flex-1 flex overflow-hidden">

                        {/* Center Canvas */}
                        <div className="flex-1 flex flex-col relative">
                            {/* Stats Bar */}
                            <div className="h-16 border-b border-white/5 bg-zinc-900/30 flex items-center px-6 gap-8 overflow-x-auto">
                                <StatCard label="Indexed Nodes" value={statsLoading ? '...' : stats?.total_entities || 0} />
                                <div className="w-px h-8 bg-white/10"></div>
                                <StatCard label="Edge Density" value={statsLoading ? '...' : stats?.active_relations || 0} color="text-purple-400" />
                                <div className="w-px h-8 bg-white/10"></div>
                                <StatCard label="Memory Usage" value={statsLoading ? '...' : stats?.memory_usage || "0 MB"} color="text-cyan-400" />
                            </div>

                            {/* Graph */}
                            <div className="flex-1 relative bg-[#0a0a10]">
                                {graphLoading ? (
                                    <div className="absolute inset-0 flex items-center justify-center">
                                        <div className="text-zinc-500">Loading graph...</div>
                                    </div>
                                ) : graphData && (
                                    <MemoryGraph2D
                                        nodes={graphData.nodes}
                                        edges={graphData.edges}
                                        onNodeSelect={setSelectedNode}
                                        selectedNodeId={selectedNode?.id}
                                    />
                                )}

                                {/* Node Details Panel */}
                                <NodeDetailsPanel
                                    node={selectedNode}
                                    onClose={() => setSelectedNode(null)}
                                    onExpandNode={(nodeId) => {
                                        log(`Expanding connections for ${nodeId}...`);
                                        spreadActivationMutation.mutate(nodeId);
                                    }}
                                />
                            </div>
                        </div>

                        {/* Right Panel: Kernel Operations */}
                        <div className="w-80 border-l border-white/10 bg-zinc-900/50 flex flex-col backdrop-blur-sm z-10 transition-all">
                            <div className="p-4 border-b border-white/5 font-medium text-sm flex items-center justify-between">
                                <span>Kernel Operations</span>
                                <div className="w-1.5 h-1.5 rounded-full bg-green-500"></div>
                            </div>

                            <div className="p-4 space-y-6 overflow-y-auto flex-1">

                                {/* Spreading Activation */}
                                <div className="space-y-3">
                                    <div className="flex items-center justify-between">
                                        <label className="text-xs font-semibold text-zinc-400 uppercase tracking-wide">Spreading Activation</label>
                                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-400 border border-purple-500/30">v2.1</span>
                                    </div>
                                    <div className="p-3 bg-zinc-950/50 rounded-lg border border-white/5 space-y-3">
                                        <div className="space-y-1">
                                            <span className="text-xs text-zinc-500">Start Node</span>
                                            <input
                                                type="text"
                                                value={startNode}
                                                onChange={(e) => setStartNode(e.target.value)}
                                                placeholder="e.g. Akash Kesavan"
                                                className="w-full bg-zinc-900 border border-white/10 rounded text-xs px-2 py-1.5 text-zinc-300"
                                            />
                                        </div>
                                        <button
                                            onClick={handleSpreadActivation}
                                            disabled={spreadActivationMutation.isPending}
                                            className="w-full py-1.5 bg-purple-600 hover:bg-purple-500 disabled:opacity-50 text-white text-xs font-medium rounded transition shadow-lg shadow-purple-900/20"
                                        >
                                            {spreadActivationMutation.isPending ? '⏳ Running...' : '⚡ Ignite Activation'}
                                        </button>
                                    </div>
                                </div>

                                {/* Community Detection */}
                                <div className="space-y-3">
                                    <label className="text-xs font-semibold text-zinc-400 uppercase tracking-wide">Community Analysis</label>
                                    <div className="p-3 bg-zinc-950/50 rounded-lg border border-white/5 space-y-3">
                                        <div className="flex gap-2">
                                            <span className="text-[10px] bg-zinc-800 px-2 py-1 rounded text-zinc-400">Leiden</span>
                                            <span className="text-[10px] bg-zinc-800 px-2 py-1 rounded text-zinc-400">Modularity</span>
                                        </div>
                                        <button
                                            onClick={handleCommunityDetection}
                                            disabled={communityMutation.isPending}
                                            className="w-full py-1.5 bg-zinc-800 hover:bg-zinc-700 disabled:opacity-50 border border-white/5 text-zinc-200 text-xs font-medium rounded transition"
                                        >
                                            {communityMutation.isPending ? '⏳ Detecting...' : 'Detect Clusters'}
                                        </button>
                                    </div>
                                </div>

                                {/* Temporal Decay */}
                                <div className="space-y-3">
                                    <label className="text-xs font-semibold text-zinc-400 uppercase tracking-wide">Temporal Decay</label>
                                    <div className="p-3 bg-zinc-950/50 rounded-lg border border-white/5 space-y-3">
                                        <div className="flex justify-between text-xs text-zinc-500">
                                            <span>Lookback</span>
                                            <span>{lookbackDays} Days</span>
                                        </div>
                                        <input
                                            type="range"
                                            min="7"
                                            max="90"
                                            value={lookbackDays}
                                            onChange={(e) => setLookbackDays(Number(e.target.value))}
                                            className="w-full h-1 bg-zinc-800 rounded-lg appearance-none cursor-pointer"
                                        />
                                        <button
                                            onClick={handleTemporalDecay}
                                            disabled={temporalMutation.isPending}
                                            className="w-full py-1.5 bg-zinc-800 hover:bg-zinc-700 disabled:opacity-50 border border-white/5 text-zinc-200 text-xs font-medium rounded transition"
                                        >
                                            {temporalMutation.isPending ? '⏳ Applying...' : 'Apply Decay'}
                                        </button>
                                    </div>
                                </div>

                                {/* Operation Log */}
                                <div className="space-y-3">
                                    <label className="text-xs font-semibold text-zinc-400 uppercase tracking-wide">Console</label>
                                    <div className="p-3 bg-black/50 rounded-lg border border-white/5 h-24 overflow-y-auto font-mono text-[10px] text-zinc-500 space-y-1">
                                        {operationLog.length === 0 ? (
                                            <div className="text-zinc-600">Ready for operations...</div>
                                        ) : (
                                            operationLog.map((msg, i) => (
                                                <div key={i} className="text-zinc-400">{msg}</div>
                                            ))
                                        )}
                                    </div>
                                </div>

                                {/* Recent Insights */}
                                <div className="space-y-3">
                                    <div className="flex items-center gap-2">
                                        <Lightbulb className="w-3 h-3 text-yellow-500" />
                                        <label className="text-xs font-semibold text-zinc-400 uppercase tracking-wide">Recent Insights</label>
                                    </div>
                                    <div className="space-y-2">
                                        <div className="p-2.5 bg-zinc-950/50 rounded-lg border border-white/5 flex items-start gap-2">
                                            <AlertTriangle className="w-3 h-3 text-amber-500 mt-0.5 flex-shrink-0" />
                                            <div>
                                                <p className="text-xs text-zinc-300">Potential conflict detected</p>
                                                <p className="text-[10px] text-zinc-500">Thai food + peanut allergy</p>
                                            </div>
                                        </div>
                                        <div className="p-2.5 bg-zinc-950/50 rounded-lg border border-white/5 flex items-start gap-2">
                                            <Calendar className="w-3 h-3 text-blue-400 mt-0.5 flex-shrink-0" />
                                            <div>
                                                <p className="text-xs text-zinc-300">Pattern recognized</p>
                                                <p className="text-[10px] text-zinc-500">Monday planning routine</p>
                                            </div>
                                        </div>
                                    </div>
                                </div>

                                {/* Top Entities */}
                                <div className="space-y-3">
                                    <div className="flex items-center gap-2">
                                        <TrendingUp className="w-3 h-3 text-green-500" />
                                        <label className="text-xs font-semibold text-zinc-400 uppercase tracking-wide">Top Entities</label>
                                    </div>
                                    <div className="space-y-1">
                                        {[
                                            { name: 'Alex Johnson', score: 0.92, type: 'Person' },
                                            { name: 'Acme Corp', score: 0.85, type: 'Department' },
                                            { name: 'React', score: 0.78, type: 'Skill' },
                                        ].map((entity) => (
                                            <div
                                                key={entity.name}
                                                className="flex items-center justify-between px-2.5 py-2 bg-zinc-950/50 rounded-lg border border-white/5 hover:bg-zinc-800/50 cursor-pointer transition-colors"
                                                onClick={() => log(`Selected: ${entity.name}`)}
                                            >
                                                <div className="flex items-center gap-2">
                                                    <span className={cn(
                                                        "w-2 h-2 rounded-full",
                                                        entity.type === 'Person' && "bg-purple-500",
                                                        entity.type === 'Department' && "bg-orange-500",
                                                        entity.type === 'Skill' && "bg-cyan-500"
                                                    )} />
                                                    <span className="text-xs text-zinc-300">{entity.name}</span>
                                                </div>
                                                <span className="text-[10px] font-mono text-zinc-500">{entity.score.toFixed(2)}</span>
                                            </div>
                                        ))}
                                    </div>
                                </div>

                            </div>
                        </div>
                    </div>

                    {/* Footer: Ingestion Status */}
                    <div className="h-8 border-t border-white/10 bg-[#07070a] flex items-center px-4 justify-between text-[10px] text-zinc-500 font-mono">
                        <div className="flex items-center gap-4">
                            <span className="flex items-center gap-1.5">
                                <span className={cn("w-1.5 h-1.5 rounded-full", ingestion?.nats_connected ? "bg-green-500" : "bg-red-500")}></span>
                                NATS: {ingestion?.nats_connected ? "Connected" : "Disconnected"}
                            </span>
                            <span className="flex items-center gap-1.5">
                                <span className={cn("w-1.5 h-1.5 rounded-full", ingestion?.pipeline_active ? "bg-blue-500" : "bg-zinc-500")}></span>
                                Pipeline: {ingestion?.pipeline_active ? "Active" : "Paused"}
                            </span>
                        </div>
                        <div className="flex items-center gap-4">
                            <span>Processed: <span className="text-zinc-300">{ingestion?.total_processed || 0}</span></span>
                            <span>Latency: <span className="text-zinc-300">{ingestion?.avg_latency_ms || 0}ms</span></span>
                        </div>
                    </div>
                </main>

            </div>
        </div>
    );
};
