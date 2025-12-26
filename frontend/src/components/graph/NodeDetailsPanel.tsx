import React from 'react';
import { X, Clock, Zap, Link2, Tag, ChevronRight } from 'lucide-react';
import { GraphNode } from './MemoryGraph2D';

interface NodeDetailsPanelProps {
    node: GraphNode | null;
    onClose: () => void;
    onExpandNode?: (nodeId: string) => void;
}

const GROUP_COLORS: Record<string, string> = {
    Person: '#a855f7',
    Skill: '#06b6d4',
    Location: '#22c55e',
    Department: '#f97316',
    Entity: '#8b5cf6',
    Insight: '#ec4899',
    Pattern: '#14b8a6',
};

export const NodeDetailsPanel: React.FC<NodeDetailsPanelProps> = ({
    node,
    onClose,
    onExpandNode
}) => {
    if (!node) return null;

    const color = GROUP_COLORS[node.group || 'Entity'] || GROUP_COLORS.Entity;
    const activationPercent = Math.round((node.activation || 0.5) * 100);

    return (
        <div className="absolute top-4 right-20 z-20 w-72 bg-zinc-900/95 backdrop-blur-xl border border-white/10 rounded-xl shadow-2xl overflow-hidden">
            {/* Header */}
            <div
                className="px-4 py-3 flex items-center justify-between"
                style={{
                    background: `linear-gradient(135deg, ${color}20 0%, transparent 100%)`,
                    borderBottom: `1px solid ${color}40`
                }}
            >
                <div className="flex items-center gap-2">
                    <div
                        className="w-3 h-3 rounded-full"
                        style={{
                            backgroundColor: color,
                            boxShadow: `0 0 10px ${color}`
                        }}
                    />
                    <span className="text-xs font-medium text-zinc-400 uppercase tracking-wide">
                        {node.group || 'Entity'}
                    </span>
                </div>
                <button
                    onClick={onClose}
                    className="p-1 hover:bg-white/10 rounded transition-colors"
                >
                    <X className="w-4 h-4 text-zinc-400" />
                </button>
            </div>

            {/* Content */}
            <div className="p-4 space-y-4">
                {/* Node name */}
                <div>
                    <h3 className="text-lg font-semibold text-white truncate">
                        {node.label || node.id}
                    </h3>
                    <p className="text-xs text-zinc-500 font-mono">{node.id}</p>
                </div>

                {/* Stats */}
                <div className="grid grid-cols-2 gap-3">
                    <div className="bg-zinc-800/50 rounded-lg p-3">
                        <div className="flex items-center gap-2 mb-1">
                            <Zap className="w-3 h-3 text-yellow-500" />
                            <span className="text-xs text-zinc-500">Activation</span>
                        </div>
                        <div className="flex items-center gap-2">
                            <div className="flex-1 h-1.5 bg-zinc-700 rounded-full overflow-hidden">
                                <div
                                    className="h-full rounded-full transition-all duration-300"
                                    style={{
                                        width: `${activationPercent}%`,
                                        background: `linear-gradient(90deg, ${color} 0%, ${color}80 100%)`
                                    }}
                                />
                            </div>
                            <span className="text-xs font-mono text-zinc-300">{activationPercent}%</span>
                        </div>
                    </div>
                    <div className="bg-zinc-800/50 rounded-lg p-3">
                        <div className="flex items-center gap-2 mb-1">
                            <Clock className="w-3 h-3 text-blue-400" />
                            <span className="text-xs text-zinc-500">Last Access</span>
                        </div>
                        <span className="text-xs font-mono text-zinc-300">
                            {node.lastAccessed ? new Date(node.lastAccessed).toLocaleDateString() : 'Never'}
                        </span>
                    </div>
                </div>

                {/* Properties */}
                {node.properties && Object.keys(node.properties).length > 0 && (
                    <div>
                        <div className="flex items-center gap-2 mb-2">
                            <Tag className="w-3 h-3 text-zinc-500" />
                            <span className="text-xs font-medium text-zinc-400 uppercase tracking-wide">Properties</span>
                        </div>
                        <div className="space-y-1 max-h-32 overflow-y-auto">
                            {Object.entries(node.properties).map(([key, value]) => (
                                <div key={key} className="flex justify-between text-xs bg-zinc-800/30 rounded px-2 py-1.5">
                                    <span className="text-zinc-500">{key}</span>
                                    <span className="text-zinc-300 truncate max-w-[120px]">
                                        {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                                    </span>
                                </div>
                            ))}
                        </div>
                    </div>
                )}

                {/* Actions */}
                <div className="pt-2 border-t border-white/5 space-y-2">
                    <button
                        onClick={() => onExpandNode?.(node.id)}
                        className="w-full flex items-center justify-between px-3 py-2 bg-zinc-800/50 hover:bg-zinc-800 rounded-lg transition-colors group"
                    >
                        <div className="flex items-center gap-2">
                            <Link2 className="w-4 h-4 text-zinc-500 group-hover:text-purple-400" />
                            <span className="text-sm text-zinc-300">Expand connections</span>
                        </div>
                        <ChevronRight className="w-4 h-4 text-zinc-500 group-hover:text-purple-400 group-hover:translate-x-0.5 transition-transform" />
                    </button>
                </div>
            </div>
        </div>
    );
};
