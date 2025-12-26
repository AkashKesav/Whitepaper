import React, { useRef, useEffect, useCallback, useState, useMemo } from 'react';
import ForceGraph2D, { ForceGraphMethods } from 'react-force-graph-2d';
import { Plus, Minus, Maximize2, Eye, EyeOff, Search, X, Filter, Focus, Trash2, Share2 } from 'lucide-react';
import { cn } from '@/lib/utils';

// Types
export interface GraphNode {
    id: string;
    label?: string;
    group?: string;
    size?: number;
    activation?: number;
    lastAccessed?: string;
    properties?: Record<string, any>;
}

export interface GraphEdge {
    source: string;
    target: string;
    id?: string;
    label?: string;
    weight?: number;
}

interface MemoryGraph2DProps {
    nodes: GraphNode[];
    edges: GraphEdge[];
    onNodeSelect?: (node: GraphNode | null) => void;
    selectedNodeId?: string | null;
}

// Apple-inspired muted color palette
const GROUP_COLORS: Record<string, { fill: string; text: string; shadow: string }> = {
    Person: { fill: '#7C3AED', text: '#fff', shadow: 'rgba(124, 58, 237, 0.35)' },
    Skill: { fill: '#0EA5E9', text: '#fff', shadow: 'rgba(14, 165, 233, 0.35)' },
    Location: { fill: '#10B981', text: '#fff', shadow: 'rgba(16, 185, 129, 0.35)' },
    Department: { fill: '#F59E0B', text: '#fff', shadow: 'rgba(245, 158, 11, 0.35)' },
    Entity: { fill: '#6366F1', text: '#fff', shadow: 'rgba(99, 102, 241, 0.35)' },
    Insight: { fill: '#EC4899', text: '#fff', shadow: 'rgba(236, 72, 153, 0.35)' },
    Pattern: { fill: '#14B8A6', text: '#fff', shadow: 'rgba(20, 184, 166, 0.35)' },
};

export const MemoryGraph2D: React.FC<MemoryGraph2DProps> = ({
    nodes,
    edges,
    onNodeSelect,
    selectedNodeId
}) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const graphRef = useRef<ForceGraphMethods>();
    const [hoveredNode, setHoveredNode] = useState<string | null>(null);
    const [showLabels, setShowLabels] = useState(true);
    const [localSearch, setLocalSearch] = useState('');
    const [dimensions, setDimensions] = useState({ width: 800, height: 600 });
    const [contextMenu, setContextMenu] = useState<{ x: number; y: number; node: any } | null>(null);
    const [typeFilter, setTypeFilter] = useState<string | null>(null);
    const [showFilterMenu, setShowFilterMenu] = useState(false);

    // Resize observer
    useEffect(() => {
        if (!containerRef.current) return;
        const resizeObserver = new ResizeObserver((entries) => {
            const { width, height } = entries[0].contentRect;
            setDimensions({ width, height });
        });
        resizeObserver.observe(containerRef.current);
        return () => resizeObserver.disconnect();
    }, []);

    // Transform data with type filtering
    const graphData = useMemo(() => {
        const filteredNodes = nodes
            .filter(n => !typeFilter || (n.group === typeFilter))
            .map(n => ({
                id: n.id,
                name: n.label || n.id,
                group: n.group || 'Entity',
                val: (n.size || 10) * Math.max(0.5, n.activation || 0.5),
                activation: n.activation || 0.5,
                originalData: n,
            }));

        const nodeIds = new Set(filteredNodes.map(n => n.id));

        return {
            nodes: filteredNodes,
            links: edges
                .filter(e => nodeIds.has(e.source) && nodeIds.has(e.target))
                .map(e => ({
                    source: e.source,
                    target: e.target,
                    label: e.label || '',
                })),
        };
    }, [nodes, edges, typeFilter]);

    // Get connected nodes
    const getConnectedNodes = useCallback((nodeId: string): Set<string> => {
        const connected = new Set<string>([nodeId]);
        edges.forEach(edge => {
            if (edge.source === nodeId) connected.add(edge.target);
            if (edge.target === nodeId) connected.add(edge.source);
        });
        return connected;
    }, [edges]);

    // Search match
    const matchesSearch = useCallback((node: any): boolean => {
        if (!localSearch) return false;
        return node.name.toLowerCase().includes(localSearch.toLowerCase());
    }, [localSearch]);

    // Get colors
    const getColors = useCallback((group: string) => {
        return GROUP_COLORS[group] || GROUP_COLORS.Entity;
    }, []);

    // Apple-style node drawing
    const drawNode = useCallback((node: any, ctx: CanvasRenderingContext2D, globalScale: number) => {
        if (node.x === undefined || node.y === undefined) return;

        const colors = getColors(node.group);
        const isSelected = node.id === selectedNodeId;
        const isHovered = node.id === hoveredNode;
        const isConnected = hoveredNode ? getConnectedNodes(hoveredNode).has(node.id) : true;
        const isSearchMatch = matchesSearch(node);
        const isDimmed = hoveredNode && !isConnected;

        const baseRadius = Math.sqrt(node.val || 10) * 2.2;
        const radius = isHovered || isSelected ? baseRadius * 1.1 : baseRadius;

        ctx.save();

        // Soft shadow for depth (Apple style)
        if (!isDimmed) {
            ctx.shadowColor = colors.shadow;
            ctx.shadowBlur = isSelected ? 20 : (isHovered ? 15 : 8);
            ctx.shadowOffsetX = 0;
            ctx.shadowOffsetY = isSelected ? 4 : 2;
        }

        // Main circle - clean solid color
        ctx.beginPath();
        ctx.arc(node.x, node.y, radius, 0, Math.PI * 2);
        ctx.fillStyle = isDimmed ? `${colors.fill}30` : colors.fill;
        ctx.fill();

        // Reset shadow for border
        ctx.shadowColor = 'transparent';
        ctx.shadowBlur = 0;

        // Subtle inner light (Apple glass effect)
        if (!isDimmed) {
            const gradient = ctx.createLinearGradient(
                node.x - radius, node.y - radius,
                node.x + radius, node.y + radius
            );
            gradient.addColorStop(0, 'rgba(255, 255, 255, 0.25)');
            gradient.addColorStop(0.5, 'rgba(255, 255, 255, 0.05)');
            gradient.addColorStop(1, 'rgba(0, 0, 0, 0.1)');
            ctx.beginPath();
            ctx.arc(node.x, node.y, radius, 0, Math.PI * 2);
            ctx.fillStyle = gradient;
            ctx.fill();
        }

        // Selection ring - thin and elegant
        if (isSelected) {
            ctx.beginPath();
            ctx.arc(node.x, node.y, radius + 4, 0, Math.PI * 2);
            ctx.strokeStyle = colors.fill;
            ctx.lineWidth = 1.5;
            ctx.stroke();
        }

        // Search highlight ring
        if (isSearchMatch && !isSelected) {
            ctx.beginPath();
            ctx.arc(node.x, node.y, radius + 3, 0, Math.PI * 2);
            ctx.strokeStyle = '#F59E0B';
            ctx.lineWidth = 2;
            ctx.stroke();
        }

        // Clean label - Apple SF Pro style (small, consistent size in canvas units)
        if (showLabels && !isDimmed) {
            // Fixed small size in canvas units - appears proportional to nodes
            const fontSize = 3;
            ctx.font = `500 ${fontSize}px -apple-system, BlinkMacSystemFont, "SF Pro Display", system-ui, sans-serif`;
            ctx.textAlign = 'center';
            ctx.textBaseline = 'top';

            // Main label
            ctx.fillStyle = isDimmed ? 'rgba(255,255,255,0.3)' : 'rgba(255, 255, 255, 0.85)';
            ctx.fillText(node.name, node.x, node.y + radius + 1);
        }

        ctx.restore();
    }, [selectedNodeId, hoveredNode, showLabels, getColors, getConnectedNodes, matchesSearch]);

    // Apple-style link drawing - subtle and clean
    const drawLink = useCallback((link: any, ctx: CanvasRenderingContext2D, globalScale: number) => {
        const start = link.source;
        const end = link.target;
        if (!start.x || !end.x) return;

        const isConnectedToHovered = hoveredNode &&
            (start.id === hoveredNode || end.id === hoveredNode);
        const isDimmed = hoveredNode && !isConnectedToHovered;

        ctx.save();

        // Simple straight line (Apple prefers clean geometry)
        ctx.beginPath();
        ctx.moveTo(start.x, start.y);
        ctx.lineTo(end.x, end.y);

        if (isConnectedToHovered) {
            ctx.strokeStyle = 'rgba(255, 255, 255, 0.5)';
            ctx.lineWidth = 1.5;
        } else {
            ctx.strokeStyle = isDimmed ? 'rgba(255, 255, 255, 0.03)' : 'rgba(255, 255, 255, 0.08)';
            ctx.lineWidth = 1;
        }
        ctx.stroke();

        ctx.restore();
    }, [hoveredNode]);

    // Handlers
    const handleNodeClick = useCallback((node: any) => {
        onNodeSelect?.(node.originalData);
    }, [onNodeSelect]);

    const handleNodeHover = useCallback((node: any) => {
        setHoveredNode(node?.id || null);
        if (containerRef.current) {
            containerRef.current.style.cursor = node ? 'pointer' : 'default';
        }
    }, []);

    const handleBackgroundClick = useCallback(() => {
        onNodeSelect?.(null);
        setContextMenu(null);
    }, [onNodeSelect]);

    // Right-click handler
    const handleNodeRightClick = useCallback((node: any, event: MouseEvent) => {
        event.preventDefault();
        setContextMenu({
            x: event.clientX,
            y: event.clientY,
            node: node
        });
    }, []);

    // Controls
    const zoomIn = () => graphRef.current?.zoom((graphRef.current?.zoom() || 1) * 1.4, 300);
    const zoomOut = () => graphRef.current?.zoom((graphRef.current?.zoom() || 1) / 1.4, 300);
    const resetView = () => graphRef.current?.zoomToFit(300, 80);

    // Initial zoom
    useEffect(() => {
        if (graphRef.current) {
            setTimeout(() => graphRef.current?.zoomToFit(400, 80), 300);
        }
    }, [nodes, edges]);

    return (
        <div ref={containerRef} className="absolute inset-0 overflow-hidden">
            {/* Clean dark background with subtle gradient */}
            <div
                className="absolute inset-0"
                style={{
                    background: 'linear-gradient(180deg, #1C1C1E 0%, #000000 100%)'
                }}
            />

            {/* Apple-style frosted glass controls */}
            <div className="absolute top-5 right-5 z-20">
                <div
                    className="flex flex-col backdrop-blur-xl rounded-2xl p-1.5 shadow-lg"
                    style={{
                        background: 'rgba(44, 44, 46, 0.72)',
                        border: '0.5px solid rgba(255, 255, 255, 0.1)'
                    }}
                >
                    <button
                        onClick={zoomIn}
                        className="p-3 rounded-xl transition-all active:scale-95 hover:bg-white/10"
                        title="Zoom in"
                    >
                        <Plus className="w-4 h-4 text-white/80" strokeWidth={2} />
                    </button>
                    <button
                        onClick={zoomOut}
                        className="p-3 rounded-xl transition-all active:scale-95 hover:bg-white/10"
                        title="Zoom out"
                    >
                        <Minus className="w-4 h-4 text-white/80" strokeWidth={2} />
                    </button>
                    <div className="mx-2 my-1 h-px bg-white/10" />
                    <button
                        onClick={resetView}
                        className="p-3 rounded-xl transition-all active:scale-95 hover:bg-white/10"
                        title="Fit to view"
                    >
                        <Maximize2 className="w-4 h-4 text-white/80" strokeWidth={2} />
                    </button>
                    <button
                        onClick={() => setShowLabels(!showLabels)}
                        className={`p-3 rounded-xl transition-all active:scale-95 ${showLabels ? 'bg-white/15' : 'hover:bg-white/10'}`}
                        title="Toggle labels"
                    >
                        {showLabels ?
                            <Eye className="w-4 h-4 text-white/80" strokeWidth={2} /> :
                            <EyeOff className="w-4 h-4 text-white/60" strokeWidth={2} />
                        }
                    </button>
                </div>
            </div>

            {/* Apple-style search */}
            <div className="absolute top-5 left-5 z-20">
                <div
                    className="flex items-center backdrop-blur-xl rounded-xl px-4 py-3 gap-3 shadow-lg"
                    style={{
                        background: 'rgba(44, 44, 46, 0.72)',
                        border: '0.5px solid rgba(255, 255, 255, 0.1)',
                        minWidth: '220px'
                    }}
                >
                    <Search className="w-4 h-4 text-white/40" strokeWidth={2} />
                    <input
                        type="text"
                        value={localSearch}
                        onChange={(e) => setLocalSearch(e.target.value)}
                        placeholder="Search"
                        className="bg-transparent text-sm text-white/90 placeholder-white/40 outline-none flex-1"
                        style={{ fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", system-ui' }}
                    />
                    {localSearch && (
                        <button
                            onClick={() => setLocalSearch('')}
                            className="p-1 rounded-full bg-white/20 hover:bg-white/30 transition-colors"
                        >
                            <X className="w-3 h-3 text-white/80" strokeWidth={2.5} />
                        </button>
                    )}
                </div>
            </div>

            {/* Graph */}
            <ForceGraph2D
                ref={graphRef}
                graphData={graphData}
                width={dimensions.width}
                height={dimensions.height}
                nodeCanvasObject={drawNode}
                nodePointerAreaPaint={(node: any, color: string, ctx: CanvasRenderingContext2D) => {
                    ctx.beginPath();
                    ctx.arc(node.x, node.y, Math.sqrt(node.val || 10) * 3, 0, Math.PI * 2);
                    ctx.fillStyle = color;
                    ctx.fill();
                }}
                linkCanvasObject={drawLink}
                onNodeClick={handleNodeClick}
                onNodeHover={handleNodeHover}
                onNodeRightClick={handleNodeRightClick}
                onBackgroundClick={handleBackgroundClick}
                backgroundColor="transparent"
                cooldownTicks={100}
                d3AlphaDecay={0.025}
                d3VelocityDecay={0.2}
                warmupTicks={50}
                enableNodeDrag={true}
                enablePanInteraction={true}
                enableZoomInteraction={true}
            />

            {/* Clickable legend for filtering */}
            <div
                className="absolute bottom-5 left-5 z-20 backdrop-blur-xl rounded-2xl p-4 shadow-lg"
                style={{
                    background: 'rgba(44, 44, 46, 0.72)',
                    border: '0.5px solid rgba(255, 255, 255, 0.1)'
                }}
            >
                <div className="flex items-center justify-between mb-3">
                    <span
                        className="text-[11px] text-white/40 uppercase tracking-wider font-medium"
                        style={{ fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", system-ui' }}
                    >
                        Filter by Type
                    </span>
                    {typeFilter && (
                        <button
                            onClick={() => setTypeFilter(null)}
                            className="text-[10px] text-blue-400 hover:text-blue-300"
                        >
                            Clear
                        </button>
                    )}
                </div>
                <div className="grid grid-cols-2 gap-x-5 gap-y-2">
                    {Object.entries(GROUP_COLORS).slice(0, 6).map(([type, colors]) => (
                        <button
                            key={type}
                            onClick={() => setTypeFilter(typeFilter === type ? null : type)}
                            className={cn(
                                "flex items-center gap-2.5 px-1 py-0.5 rounded transition-all",
                                typeFilter === type ? "bg-white/10" : "hover:bg-white/5"
                            )}
                        >
                            <span
                                className="w-2.5 h-2.5 rounded-full"
                                style={{
                                    backgroundColor: colors.fill,
                                    boxShadow: `0 2px 6px ${colors.shadow}`
                                }}
                            />
                            <span
                                className={cn("text-xs", typeFilter === type ? "text-white" : "text-white/70")}
                                style={{ fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", system-ui' }}
                            >
                                {type}
                            </span>
                        </button>
                    ))}
                </div>
            </div>

            {/* Apple-style stats pill */}
            <div
                className="absolute bottom-5 right-5 z-20 backdrop-blur-xl rounded-full px-4 py-2 shadow-lg flex items-center gap-3"
                style={{
                    background: 'rgba(44, 44, 46, 0.72)',
                    border: '0.5px solid rgba(255, 255, 255, 0.1)',
                    fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", system-ui'
                }}
            >
                <span className="text-xs">
                    <span className="text-white/90 font-medium">{nodes.length}</span>
                    <span className="text-white/40 ml-1">nodes</span>
                </span>
                <span className="w-px h-3 bg-white/20" />
                <span className="text-xs">
                    <span className="text-white/90 font-medium">{edges.length}</span>
                    <span className="text-white/40 ml-1">edges</span>
                </span>
            </div>

            {/* Context Menu */}
            {contextMenu && (
                <div
                    className="fixed z-50 backdrop-blur-xl rounded-xl shadow-2xl py-1 min-w-[160px]"
                    style={{
                        left: contextMenu.x,
                        top: contextMenu.y,
                        background: 'rgba(44, 44, 46, 0.95)',
                        border: '0.5px solid rgba(255, 255, 255, 0.15)'
                    }}
                >
                    <button
                        onClick={() => { onNodeSelect?.(contextMenu.node.originalData); setContextMenu(null); }}
                        className="w-full px-4 py-2 text-left text-sm text-white/90 hover:bg-white/10 flex items-center gap-2"
                    >
                        <Focus className="w-4 h-4 text-white/50" />
                        View Details
                    </button>
                    <button
                        onClick={() => { setTypeFilter(contextMenu.node.group); setContextMenu(null); }}
                        className="w-full px-4 py-2 text-left text-sm text-white/90 hover:bg-white/10 flex items-center gap-2"
                    >
                        <Filter className="w-4 h-4 text-white/50" />
                        Filter by Type
                    </button>
                    <div className="h-px bg-white/10 my-1" />
                    <button
                        onClick={() => { setContextMenu(null); }}
                        className="w-full px-4 py-2 text-left text-sm text-red-400 hover:bg-red-500/10 flex items-center gap-2"
                    >
                        <Trash2 className="w-4 h-4" />
                        Remove from View
                    </button>
                </div>
            )}
        </div>
    );
};
