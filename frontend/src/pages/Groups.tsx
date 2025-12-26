import React, { useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { api } from '@/lib/api'; // Added import
import { cn } from '@/lib/utils';
import {
    Users, Plus, Search, MoreHorizontal, Settings,
    UserPlus, Trash2, Edit2, ChevronRight, Shield, Globe
} from 'lucide-react';

interface Group {
    id: string;
    name: string;
    description: string;
    memberCount: number;
    visibility: 'private' | 'public';
    createdAt: Date;
    color: string;
}

interface Member {
    id: string;
    name: string;
    email: string;
    role: 'owner' | 'admin' | 'member';
    avatar?: string;
}

const mockGroups: Group[] = [
    { id: '1', name: 'Engineering Team', description: 'Core engineering discussions and knowledge sharing', memberCount: 12, visibility: 'private', createdAt: new Date(), color: '#7C3AED' },
    { id: '2', name: 'Product Research', description: 'User research and product insights', memberCount: 8, visibility: 'private', createdAt: new Date(), color: '#0EA5E9' },
    { id: '3', name: 'Company Updates', description: 'Organization-wide announcements', memberCount: 45, visibility: 'public', createdAt: new Date(), color: '#10B981' },
];

const mockMembers: Member[] = [
    { id: '1', name: 'Alex Johnson', email: 'alex@example.com', role: 'owner' },
    { id: '2', name: 'Sarah Chen', email: 'sarah@example.com', role: 'admin' },
    { id: '3', name: 'Mike Wilson', email: 'mike@example.com', role: 'member' },
    { id: '4', name: 'Emily Davis', email: 'emily@example.com', role: 'member' },
];

export const Groups: React.FC = () => {
    const { user } = useAuth();
    const [groups, setGroups] = useState<Group[]>([]);
    const [selectedGroup, setSelectedGroup] = useState<string | null>(null);
    const [searchQuery, setSearchQuery] = useState('');
    const [showCreateModal, setShowCreateModal] = useState(false); // Can implement modal later if needed

    // Fetch groups on mount
    React.useEffect(() => {
        const fetchGroups = async () => {
            const data = await api.getGroups();
            // Transform backend data to frontend model
            // Backend returns Namespace strings usually?
            // Let's assume for now backend returns objects with name/description/id
            // If backend returns raw namespaces (strings), we need to adapt.
            // Based on handleListGroups, it returns map "groups": []string or []Struct
            // handleListGroups calls mkClient.ListGroups(ctx, userID)
            // mkClient.ListGroups returns meta-nodes (Group nodes).
            // Let's map it safely.

            const mappedGroups: Group[] = data.map((g: any) => ({
                id: g.uid || g.namespace || g.name, // Fallback
                name: g.name || g.namespace,
                description: g.description || "No description",
                memberCount: g.member_count || 1, // Default to self
                visibility: 'private', // API doesn't expose visibility yet?
                createdAt: new Date(),
                color: '#' + Math.floor(Math.random() * 16777215).toString(16) // Random color per group
            }));

            setGroups(mappedGroups);
            if (mappedGroups.length > 0 && !selectedGroup) {
                setSelectedGroup(mappedGroups[0].id);
            }
        };
        fetchGroups();
    }, [user]);

    const handleCreateGroup = async () => {
        const name = prompt("Enter group name:");
        if (!name) return;
        const description = prompt("Enter description:");

        try {
            const newGroup = await api.createGroup(name, description || "");
            // Refresh list
            window.location.reload(); // Simple reload for now or fetchGroups()
        } catch (e) {
            alert("Failed to create group");
        }
    };

    const filteredGroups = groups.filter(g =>
        g.name.toLowerCase().includes(searchQuery.toLowerCase())
    );

    const currentGroup = groups.find(g => g.id === selectedGroup);

    return (
        <div className="h-screen flex bg-[#1C1C1E]">
            {/* Sidebar */}
            <div
                className="w-80 flex flex-col border-r"
                style={{ borderColor: 'rgba(255,255,255,0.1)' }}
            >
                {/* Header */}
                <div className="p-4 border-b border-white/10">
                    <div className="flex items-center justify-between mb-4">
                        <h1 className="text-lg font-semibold text-white">Groups</h1>
                        <button
                            onClick={handleCreateGroup}
                            className="p-2 rounded-lg hover:bg-white/10 transition-colors"
                        >
                            <Plus className="w-5 h-5 text-white/60" />
                        </button>
                    </div>
                    {/* Search Input ... same ... */}
                    <div className="relative">
                        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-white/40" />
                        <input
                            type="text"
                            placeholder="Search groups"
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            className="w-full pl-10 pr-4 py-2 bg-white/5 rounded-xl text-sm text-white placeholder-white/40 outline-none focus:bg-white/10 transition-colors"
                        />
                    </div>
                </div>

                {/* Groups List */}
                <div className="flex-1 overflow-y-auto p-2">
                    {filteredGroups.map((group) => (
                        <button
                            key={group.id}
                            onClick={() => setSelectedGroup(group.id)}
                            className={cn(
                                "w-full p-3 rounded-xl text-left transition-all mb-1",
                                selectedGroup === group.id
                                    ? "bg-white/10"
                                    : "hover:bg-white/5"
                            )}
                        >
                            <div className="flex items-start gap-3">
                                <div
                                    className="w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0"
                                    style={{ backgroundColor: group.color }}
                                >
                                    <Users className="w-5 h-5 text-white" />
                                </div>
                                <div className="flex-1 min-w-0">
                                    <div className="flex items-center gap-2">
                                        <span className="font-medium text-sm text-white">
                                            {group.name}
                                        </span>
                                        {/* Status icons */}
                                    </div>
                                    <p className="text-xs text-white/50 mt-0.5">
                                        {group.memberCount} members
                                    </p>
                                </div>
                            </div>
                        </button>
                    ))}
                </div>
            </div>

            {/* Main Content */}
            <div className="flex-1 flex flex-col bg-black">
                {currentGroup ? (
                    <>
                        {/* Group Header */}
                        <div className="p-6 border-b border-white/10">
                            <div className="flex items-start justify-between">
                                <div className="flex items-center gap-4">
                                    <div
                                        className="w-14 h-14 rounded-2xl flex items-center justify-center"
                                        style={{ backgroundColor: currentGroup.color }}
                                    >
                                        <Users className="w-7 h-7 text-white" />
                                    </div>
                                    <div>
                                        <h2 className="text-xl font-semibold text-white">{currentGroup.name}</h2>
                                        <p className="text-sm text-white/50 mt-1">{currentGroup.description}</p>
                                    </div>
                                </div>
                            </div>
                        </div>

                        {/* Members - Placeholder as API for list-members isn't separate yet */}
                        <div className="flex-1 overflow-y-auto p-6">
                            <div className="text-center mt-10 text-white/40">
                                <Users className="w-12 h-12 mx-auto mb-4 opacity-50" />
                                <p>Member management coming soon via DGraph v2</p>
                            </div>
                        </div>
                    </>
                ) : (
                    <div className="flex-1 flex items-center justify-center">
                        <div className="text-center">
                            <Users className="w-12 h-12 text-white/20 mx-auto mb-4" />
                            <p className="text-white/40">Select a group to view details</p>
                            <button
                                onClick={handleCreateGroup}
                                className="mt-4 px-4 py-2 bg-blue-500 hover:bg-blue-600 rounded-lg text-sm text-white transition-colors"
                            >
                                Create New Group
                            </button>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
};

export default Groups;
