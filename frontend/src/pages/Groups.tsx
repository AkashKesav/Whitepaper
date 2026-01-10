import React, { useState, useEffect, useRef } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { api } from '@/lib/api';
import { cn } from '@/lib/utils';
import {
    Users, Plus, Search, MoreHorizontal, Settings,
    UserPlus, Trash2, Edit2, ChevronRight, Shield, Globe, X, Crown,
    MessageCircle, Send, ChevronLeft, Clock
} from 'lucide-react';

interface Group {
    id: string;
    name: string;
    description: string;
    memberCount: number;
    visibility: 'private' | 'public';
    createdAt: Date;
    color: string;
    namespace?: string;
}

interface Member {
    uid: string;
    name: string;
    username: string;
    role: 'admin' | 'member';
}

interface ChatMessage {
    id: string;
    role: 'user' | 'assistant';
    content: string;
    timestamp: Date;
}

// AddMemberModal component with user dropdown, role selection, and invitation status
interface Invitation {
    uid: string;
    workspace_ns: string;
    invitee_user_id: string;
    invited_by: string;
    role: string;
    status: string;
    created_at: string;
}

interface AddMemberModalProps {
    isOpen: boolean;
    onClose: () => void;
    allUsers: { username: string; role: string }[];
    loadingUsers: boolean;
    userSearchQuery: string;
    setUserSearchQuery: (q: string) => void;
    selectedUsername: string;
    setSelectedUsername: (u: string) => void;
    selectedRole: 'member' | 'admin';
    setSelectedRole: (r: 'member' | 'admin') => void;
    onAddMember: () => void;
    isSubmitting: boolean;
    members: Member[];
    onFetchUsers: () => void;
    pendingInvitations?: Invitation[];
    onFetchInvitations?: () => void;
    groupId?: string; // Group ID for direct add after creating user
    onMemberAdded?: () => void; // Callback to refresh members after direct add
}

const AddMemberModal: React.FC<AddMemberModalProps> = ({
    isOpen, onClose, allUsers, loadingUsers, userSearchQuery, setUserSearchQuery,
    selectedUsername, setSelectedUsername, selectedRole, setSelectedRole,
    onAddMember, isSubmitting, members, onFetchUsers, pendingInvitations = [], onFetchInvitations,
    groupId, onMemberAdded
}) => {
    const [activeTab, setActiveTab] = useState<'add' | 'pending' | 'create'>('add');
    
    // Create user form state
    const [newUsername, setNewUsername] = useState('');
    const [newPassword, setNewPassword] = useState('');
    const [newUserRole, setNewUserRole] = useState<'user' | 'admin'>('user');
    const [isCreating, setIsCreating] = useState(false);
    const [createError, setCreateError] = useState('');

    // Fetch users and invitations when modal opens
    useEffect(() => {
        if (isOpen) {
            onFetchUsers();
            onFetchInvitations?.();
        }
    }, [isOpen]);

    // Filter users based on search and exclude existing members
    const existingMemberUsernames = members.map(m => m.username);
    const pendingUsernames = pendingInvitations.map(inv => inv.invitee_user_id);
    
    const filteredUsers = allUsers.filter(u => 
        !existingMemberUsernames.includes(u.username) &&
        u.username.toLowerCase().includes(userSearchQuery.toLowerCase())
    );

    // Check if user has pending invitation
    const hasPendingInvitation = (username: string) => pendingUsernames.includes(username);

    // Handle create user - creates user and adds them directly to the group
    const handleCreateUser = async () => {
        if (!newUsername.trim() || !newPassword.trim()) return;
        
        setIsCreating(true);
        setCreateError('');
        
        try {
            await api.createUser(newUsername, newPassword, newUserRole);
            
            // If we have a group ID, add the user directly to the group (no invitation needed)
            if (groupId) {
                await api.addGroupMember(groupId, newUsername);
                alert(`User "${newUsername}" created and added to the group!`);
                onMemberAdded?.(); // Refresh members list
                onClose(); // Close modal on success
            } else {
                // No group context - just refresh users and switch tab
                onFetchUsers();
                setActiveTab('add');
                setSelectedUsername(newUsername);
            }
            
            // Clear form
            setNewUsername('');
            setNewPassword('');
            setNewUserRole('user');
        } catch (e: any) {
            setCreateError(e.message || 'Failed to create user');
        } finally {
            setIsCreating(false);
        }
    };

    if (!isOpen) return null;

    return (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
            <div className="bg-[#2C2C2E] rounded-2xl p-6 w-full max-w-md mx-4 max-h-[80vh] flex flex-col">
                {/* Header */}
                <div className="flex items-center justify-between mb-4">
                    <h3 className="text-lg font-semibold text-white">Manage Members</h3>
                    <button
                        onClick={onClose}
                        className="p-1 hover:bg-white/10 rounded-lg transition-colors"
                    >
                        <X className="w-5 h-5 text-white/60" />
                    </button>
                </div>

                {/* Tabs */}
                <div className="flex mb-4 bg-white/5 rounded-xl p-1">
                    <button
                        onClick={() => setActiveTab('add')}
                        className={cn(
                            "flex-1 py-2 px-3 rounded-lg text-xs font-medium transition-all flex items-center justify-center gap-1",
                            activeTab === 'add'
                                ? "bg-blue-500 text-white"
                                : "text-white/60 hover:text-white"
                        )}
                    >
                        <UserPlus className="w-3.5 h-3.5" />
                        Add
                    </button>
                    <button
                        onClick={() => setActiveTab('pending')}
                        className={cn(
                            "flex-1 py-2 px-3 rounded-lg text-xs font-medium transition-all flex items-center justify-center gap-1",
                            activeTab === 'pending'
                                ? "bg-orange-500 text-white"
                                : "text-white/60 hover:text-white"
                        )}
                    >
                        <Clock className="w-3.5 h-3.5" />
                        Pending
                        {pendingInvitations.length > 0 && (
                            <span className="ml-0.5 px-1 py-0.5 bg-white/20 rounded-full text-[10px]">
                                {pendingInvitations.length}
                            </span>
                        )}
                    </button>
                    <button
                        onClick={() => setActiveTab('create')}
                        className={cn(
                            "flex-1 py-2 px-3 rounded-lg text-xs font-medium transition-all flex items-center justify-center gap-1",
                            activeTab === 'create'
                                ? "bg-green-500 text-white"
                                : "text-white/60 hover:text-white"
                        )}
                    >
                        <Plus className="w-3.5 h-3.5" />
                        Create
                    </button>
                </div>

                {/* Tab Content */}
                {activeTab === 'add' ? (
                    <div className="space-y-4 flex-1 overflow-y-auto">
                        {/* Search Box */}
                        <div>
                            <label className="block text-sm text-white/60 mb-2">Search Users</label>
                            <div className="relative">
                                <Search className="w-4 h-4 text-white/40 absolute left-3 top-1/2 -translate-y-1/2" />
                                <input
                                    type="text"
                                    value={userSearchQuery}
                                    onChange={(e) => setUserSearchQuery(e.target.value)}
                                    placeholder="Search by username..."
                                    className="w-full pl-10 pr-4 py-3 bg-white/5 rounded-xl text-white placeholder-white/40 outline-none focus:ring-2 focus:ring-blue-500"
                                />
                            </div>
                        </div>

                        {/* User List */}
                        <div>
                            <label className="block text-sm text-white/60 mb-2">
                                Select User {loadingUsers && <span className="text-xs">(loading...)</span>}
                            </label>
                            <div className="bg-white/5 rounded-xl max-h-48 overflow-y-auto">
                                {loadingUsers ? (
                                    <div className="p-4 text-center text-white/40 text-sm">
                                        Loading users...
                                    </div>
                                ) : filteredUsers.length === 0 ? (
                                    <div className="p-4 text-center text-white/40 text-sm">
                                        {userSearchQuery ? 'No users match your search' : 'No available users'}
                                    </div>
                                ) : (
                                    filteredUsers.map((u) => (
                                        <button
                                            key={u.username}
                                            onClick={() => setSelectedUsername(u.username)}
                                            disabled={hasPendingInvitation(u.username)}
                                            className={cn(
                                                "w-full px-4 py-3 flex items-center justify-between text-left transition-colors",
                                                hasPendingInvitation(u.username) 
                                                    ? "opacity-50 cursor-not-allowed bg-orange-500/10" 
                                                    : "hover:bg-white/10",
                                                selectedUsername === u.username && "bg-blue-500/20 border-l-2 border-blue-500"
                                            )}
                                        >
                                            <div className="flex items-center gap-3">
                                                <div className="w-8 h-8 rounded-full bg-gradient-to-br from-purple-500 to-blue-500 flex items-center justify-center text-white font-medium text-sm">
                                                    {u.username.charAt(0).toUpperCase()}
                                                </div>
                                                <div>
                                                    <p className="text-white text-sm font-medium">{u.username}</p>
                                                    <p className="text-white/40 text-xs">
                                                        {hasPendingInvitation(u.username) 
                                                            ? '⏳ Invitation pending' 
                                                            : u.role === 'admin' ? '🛡️ Admin' : 'User'
                                                        }
                                                    </p>
                                                </div>
                                            </div>
                                            {selectedUsername === u.username && !hasPendingInvitation(u.username) && (
                                                <div className="w-2 h-2 rounded-full bg-blue-500" />
                                            )}
                                            {hasPendingInvitation(u.username) && (
                                                <span className="text-xs px-2 py-1 bg-orange-500/20 text-orange-400 rounded-full">
                                                    Pending
                                                </span>
                                            )}
                                        </button>
                                    ))
                                )}
                            </div>
                        </div>

                        {/* Role Selection */}
                        {selectedUsername && !hasPendingInvitation(selectedUsername) && (
                            <div>
                                <label className="block text-sm text-white/60 mb-2">Assign Role</label>
                                <div className="flex gap-2">
                                    <button
                                        onClick={() => setSelectedRole('member')}
                                        className={cn(
                                            "flex-1 px-4 py-3 rounded-xl flex items-center justify-center gap-2 transition-all",
                                            selectedRole === 'member'
                                                ? "bg-green-500/20 border-2 border-green-500 text-green-400"
                                                : "bg-white/5 border-2 border-transparent text-white/60 hover:bg-white/10"
                                        )}
                                    >
                                        <Users className="w-4 h-4" />
                                        <span className="font-medium">Member</span>
                                    </button>
                                    <button
                                        onClick={() => setSelectedRole('admin')}
                                        className={cn(
                                            "flex-1 px-4 py-3 rounded-xl flex items-center justify-center gap-2 transition-all",
                                            selectedRole === 'admin'
                                                ? "bg-yellow-500/20 border-2 border-yellow-500 text-yellow-400"
                                                : "bg-white/5 border-2 border-transparent text-white/60 hover:bg-white/10"
                                        )}
                                    >
                                        <Crown className="w-4 h-4" />
                                        <span className="font-medium">Admin</span>
                                    </button>
                                </div>
                            </div>
                        )}

                        {/* Selected User Preview */}
                        {selectedUsername && !hasPendingInvitation(selectedUsername) && (
                            <div className="p-3 bg-blue-500/10 border border-blue-500/30 rounded-xl">
                                <p className="text-sm text-blue-400">
                                    Adding <span className="font-bold">{selectedUsername}</span> as <span className="font-bold">{selectedRole}</span>
                                </p>
                            </div>
                        )}
                    </div>
                ) : activeTab === 'pending' ? (
                    <div className="flex-1 overflow-y-auto">
                        {pendingInvitations.length === 0 ? (
                            <div className="h-full flex items-center justify-center py-12">
                                <div className="text-center text-white/40">
                                    <Clock className="w-10 h-10 mx-auto mb-3 opacity-50" />
                                    <p className="text-sm">No pending invitations</p>
                                    <p className="text-xs mt-1">Invitations you send will appear here</p>
                                </div>
                            </div>
                        ) : (
                            <div className="space-y-2">
                                {pendingInvitations.map((inv) => (
                                    <div
                                        key={inv.uid}
                                        className="p-4 bg-white/5 rounded-xl border border-orange-500/20"
                                    >
                                        <div className="flex items-center justify-between">
                                            <div className="flex items-center gap-3">
                                                <div className="w-10 h-10 rounded-full bg-gradient-to-br from-orange-500 to-yellow-500 flex items-center justify-center text-white font-medium">
                                                    {inv.invitee_user_id.charAt(0).toUpperCase()}
                                                </div>
                                                <div>
                                                    <p className="text-white font-medium">{inv.invitee_user_id}</p>
                                                    <p className="text-xs text-white/40">
                                                        Invited as {inv.role} • {new Date(inv.created_at).toLocaleDateString()}
                                                    </p>
                                                </div>
                                            </div>
                                            <span className="px-2 py-1 bg-orange-500/20 text-orange-400 rounded-full text-xs flex items-center gap-1">
                                                <Clock className="w-3 h-3" />
                                                Pending
                                            </span>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        )}
                    </div>

                ) : (
                    /* Create User Tab */
                    <div className="space-y-4 flex-1 overflow-y-auto">
                        <div className="p-3 bg-green-500/10 border border-green-500/30 rounded-xl mb-4">
                            <p className="text-xs text-green-400 flex items-center gap-2">
                                <Shield className="w-4 h-4" />
                                Admin only: Create a new user in the system
                            </p>
                        </div>

                        {createError && (
                            <div className="p-3 bg-red-500/20 border border-red-500/30 rounded-xl">
                                <p className="text-sm text-red-400">{createError}</p>
                            </div>
                        )}

                        <div>
                            <label className="block text-sm text-white/60 mb-2">Username</label>
                            <input
                                type="text"
                                value={newUsername}
                                onChange={(e) => setNewUsername(e.target.value)}
                                placeholder="Enter username"
                                className="w-full px-4 py-3 bg-white/5 rounded-xl text-white placeholder-white/40 outline-none focus:ring-2 focus:ring-green-500"
                            />
                        </div>

                        <div>
                            <label className="block text-sm text-white/60 mb-2">Password</label>
                            <input
                                type="password"
                                value={newPassword}
                                onChange={(e) => setNewPassword(e.target.value)}
                                placeholder="Enter password"
                                className="w-full px-4 py-3 bg-white/5 rounded-xl text-white placeholder-white/40 outline-none focus:ring-2 focus:ring-green-500"
                            />
                        </div>

                        <div>
                            <label className="block text-sm text-white/60 mb-2">System Role</label>
                            <div className="flex gap-2">
                                <button
                                    onClick={() => setNewUserRole('user')}
                                    className={cn(
                                        "flex-1 px-4 py-3 rounded-xl flex items-center justify-center gap-2 transition-all",
                                        newUserRole === 'user'
                                            ? "bg-blue-500/20 border-2 border-blue-500 text-blue-400"
                                            : "bg-white/5 border-2 border-transparent text-white/60 hover:bg-white/10"
                                    )}
                                >
                                    <Users className="w-4 h-4" />
                                    <span className="font-medium">User</span>
                                </button>
                                <button
                                    onClick={() => setNewUserRole('admin')}
                                    className={cn(
                                        "flex-1 px-4 py-3 rounded-xl flex items-center justify-center gap-2 transition-all",
                                        newUserRole === 'admin'
                                            ? "bg-yellow-500/20 border-2 border-yellow-500 text-yellow-400"
                                            : "bg-white/5 border-2 border-transparent text-white/60 hover:bg-white/10"
                                    )}
                                >
                                    <Crown className="w-4 h-4" />
                                    <span className="font-medium">Admin</span>
                                </button>
                            </div>
                        </div>
                    </div>
                )}

                {/* Actions */}
                <div className="flex gap-3 pt-4 mt-4 border-t border-white/10">
                    <button
                        onClick={onClose}
                        className="flex-1 px-4 py-3 bg-white/10 hover:bg-white/20 rounded-xl text-white transition-colors"
                    >
                        Cancel
                    </button>
                    {activeTab === 'add' && (
                        <button
                            onClick={onAddMember}
                            disabled={!selectedUsername || isSubmitting || hasPendingInvitation(selectedUsername)}
                            className="flex-1 px-4 py-3 bg-blue-500 hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed rounded-xl text-white transition-colors flex items-center justify-center gap-2"
                        >
                            <UserPlus className="w-4 h-4" />
                            {isSubmitting ? 'Adding...' : 'Add Member'}
                        </button>
                    )}
                    {activeTab === 'create' && (
                        <button
                            onClick={handleCreateUser}
                            disabled={!newUsername.trim() || !newPassword.trim() || isCreating}
                            className="flex-1 px-4 py-3 bg-green-500 hover:bg-green-600 disabled:opacity-50 disabled:cursor-not-allowed rounded-xl text-white transition-colors flex items-center justify-center gap-2"
                        >
                            <Plus className="w-4 h-4" />
                            {isCreating ? 'Creating...' : 'Create User'}
                        </button>
                    )}
                </div>

            </div>
        </div>
    );
};

export const Groups: React.FC = () => {
    const { user } = useAuth();
    const [groups, setGroups] = useState<Group[]>([]);
    const [selectedGroup, setSelectedGroup] = useState<string | null>(null);
    const [searchQuery, setSearchQuery] = useState('');
    const [showCreateModal, setShowCreateModal] = useState(false);
    const [showAddMemberModal, setShowAddMemberModal] = useState(false);
    const [members, setMembers] = useState<Member[]>([]);
    const [loadingMembers, setLoadingMembers] = useState(false);
    
    // Chat state - store messages per group
    const [showChat, setShowChat] = useState(false);
    const [groupChats, setGroupChats] = useState<Record<string, ChatMessage[]>>({});
    const [chatInput, setChatInput] = useState('');
    const [isSending, setIsSending] = useState(false);
    const chatEndRef = useRef<HTMLDivElement>(null);
    
    // Get current group's chat messages
    const chatMessages = selectedGroup ? (groupChats[selectedGroup] || []) : [];
    
    // Form state
    const [newGroupName, setNewGroupName] = useState('');
    const [newGroupDesc, setNewGroupDesc] = useState('');
    const [newMemberUsername, setNewMemberUsername] = useState('');
    const [selectedRole, setSelectedRole] = useState<'member' | 'admin'>('member');
    const [isSubmitting, setIsSubmitting] = useState(false);
    
    // All users list for dropdown
    const [allUsers, setAllUsers] = useState<{username: string, role: string}[]>([]);
    const [loadingUsers, setLoadingUsers] = useState(false);
    const [userSearchQuery, setUserSearchQuery] = useState('');
    
    // Pending invitations for the current group
    const [pendingInvitations, setPendingInvitations] = useState<Invitation[]>([]);


    // Generate random color for groups
    const getRandomColor = () => '#' + Math.floor(Math.random() * 16777215).toString(16).padStart(6, '0');

    // Fetch groups on mount
    useEffect(() => {
        fetchGroups();
    }, [user]);

    // Fetch members when group is selected
    useEffect(() => {
        if (selectedGroup) {
            fetchMembers(selectedGroup);
            // Don't reset chat - keep history per group
        }
    }, [selectedGroup]);

    // Scroll chat to bottom
    useEffect(() => {
        chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }, [chatMessages]);

    const fetchGroups = async () => {
        try {
            const data = await api.getGroups();
            const mappedGroups: Group[] = data.map((g: any) => ({
                id: g.namespace || g.uid || g.name,
                name: g.name || g.namespace,
                description: g.description || "No description",
                memberCount: g.group_has_member?.length || 1,
                visibility: 'private',
                createdAt: new Date(g.created_at || Date.now()),
                color: getRandomColor(),
                namespace: g.namespace,
            }));
            setGroups(mappedGroups);
            if (mappedGroups.length > 0 && !selectedGroup) {
                setSelectedGroup(mappedGroups[0].id);
            }
        } catch (e) {
            console.error("Failed to fetch groups", e);
        }
    };

    const fetchMembers = async (groupId: string) => {
        setLoadingMembers(true);
        try {
            const data = await api.getGroupMembers(groupId);
            setMembers(data.map((m: any) => ({
                uid: m.uid,
                name: m.name || m.username,
                username: m.username || m.name,
                role: m.role || 'member',
            })));
        } catch (e) {
            console.error("Failed to fetch members", e);
            setMembers([]);
        } finally {
            setLoadingMembers(false);
        }
    };

    const handleCreateGroup = async () => {
        if (!newGroupName.trim()) return;
        setIsSubmitting(true);
        try {
            await api.createGroup(newGroupName, newGroupDesc);
            setShowCreateModal(false);
            setNewGroupName('');
            setNewGroupDesc('');
            fetchGroups();
        } catch (e) {
            alert("Failed to create group");
        } finally {
            setIsSubmitting(false);
        }
    };

    const handleAddMember = async () => {
        if (!newMemberUsername.trim() || !selectedGroup) return;
        setIsSubmitting(true);
        try {
            // Send invitation instead of directly adding member
            // The user will see this in their notifications and can accept/decline
            // Map 'member' to 'subuser' as backend uses 'admin' or 'subuser'
            const backendRole = selectedRole === 'member' ? 'subuser' : 'admin';
            await api.sendInvitation(selectedGroup, newMemberUsername, backendRole);
            setShowAddMemberModal(false);
            setNewMemberUsername('');
            setSelectedRole('member');
            alert(`Invitation sent to ${newMemberUsername}. They will see it in their notifications.`);
        } catch (e: any) {
            console.error("Invitation error:", e);
            alert(`Failed to send invitation: ${e.message || 'Unknown error'}`);
        } finally {
            setIsSubmitting(false);
        }
    };



    const handleRemoveMember = async (username: string) => {
        if (!selectedGroup) return;
        if (!confirm(`Remove ${username} from this group?`)) return;
        try {
            await api.removeGroupMember(selectedGroup, username);
            fetchMembers(selectedGroup);
        } catch (e) {
            alert("Failed to remove member");
        }
    };

    const handleDeleteGroup = async () => {
        if (!selectedGroup) return;
        if (!confirm("Are you sure you want to delete this group? This cannot be undone.")) return;
        try {
            await api.deleteGroup(selectedGroup);
            setSelectedGroup(null);
            fetchGroups();
        } catch (e) {
            alert("Failed to delete group");
        }
    };

    const handleSendMessage = async () => {
        if (!chatInput.trim() || !selectedGroup || isSending) return;
        
        const userMessage: ChatMessage = {
            id: Date.now().toString(),
            role: 'user',
            content: chatInput,
            timestamp: new Date(),
        };
        
        // Add user message to this group's chat history
        setGroupChats(prev => ({
            ...prev,
            [selectedGroup]: [...(prev[selectedGroup] || []), userMessage]
        }));
        const messageToSend = chatInput;
        setChatInput('');
        setIsSending(true);
        
        try {
            const data = await api.sendGroupMessage(messageToSend, selectedGroup);
            const aiMessage: ChatMessage = {
                id: (Date.now() + 1).toString(),
                role: 'assistant',
                content: data.response || 'Sorry, I could not generate a response.',
                timestamp: new Date(),
            };
            // Add AI response to this group's chat history
            setGroupChats(prev => ({
                ...prev,
                [selectedGroup]: [...(prev[selectedGroup] || []), aiMessage]
            }));
        } catch (error) {
            console.error('Group chat error:', error);
            const errorMessage: ChatMessage = {
                id: (Date.now() + 1).toString(),
                role: 'assistant',
                content: 'Sorry, there was an error processing your request.',
                timestamp: new Date(),
            };
            setGroupChats(prev => ({
                ...prev,
                [selectedGroup]: [...(prev[selectedGroup] || []), errorMessage]
            }));
        } finally {
            setIsSending(false);
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
                            onClick={() => setShowCreateModal(true)}
                            className="p-2 rounded-lg hover:bg-white/10 transition-colors"
                            title="Create New Group"
                        >
                            <Plus className="w-5 h-5 text-white/60" />
                        </button>
                    </div>
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
                    {filteredGroups.length === 0 ? (
                        <div className="text-center py-8 text-white/40">
                            <Users className="w-8 h-8 mx-auto mb-2 opacity-50" />
                            <p className="text-sm">No groups yet</p>
                        </div>
                    ) : (
                        filteredGroups.map((group) => (
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
                                            <span className="font-medium text-sm text-white truncate">
                                                {group.name}
                                            </span>
                                        </div>
                                        <p className="text-xs text-white/50 mt-0.5">
                                            {group.memberCount} member{group.memberCount !== 1 ? 's' : ''}
                                        </p>
                                    </div>
                                </div>
                            </button>
                        ))
                    )}
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
                                <div className="flex gap-2">
                                    <button
                                        onClick={() => setShowChat(!showChat)}
                                        className={cn(
                                            "flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors",
                                            showChat 
                                                ? "bg-green-500 text-white" 
                                                : "bg-green-500/20 text-green-400 hover:bg-green-500/30"
                                        )}
                                    >
                                        <MessageCircle className="w-4 h-4" />
                                        {showChat ? 'Close Chat' : 'Group Chat'}
                                    </button>
                                    <button
                                        onClick={() => setShowAddMemberModal(true)}
                                        className="flex items-center gap-2 px-3 py-2 bg-blue-500 hover:bg-blue-600 rounded-lg text-sm text-white transition-colors"
                                    >
                                        <UserPlus className="w-4 h-4" />
                                        Add Member
                                    </button>
                                    <button
                                        onClick={handleDeleteGroup}
                                        className="p-2 hover:bg-red-500/20 rounded-lg transition-colors"
                                        title="Delete Group"
                                    >
                                        <Trash2 className="w-5 h-5 text-red-400" />
                                    </button>
                                </div>
                            </div>
                        </div>

                        {/* Content Area - Split between Members and Chat */}
                        <div className="flex-1 flex overflow-hidden">
                            {/* Members Panel */}
                            <div className={cn(
                                "flex flex-col overflow-hidden transition-all",
                                showChat ? "w-1/2 border-r border-white/10" : "flex-1"
                            )}>
                                <div className="p-6 overflow-y-auto flex-1">
                                    <h3 className="text-sm font-medium text-white/60 mb-4">
                                        Members ({members.length})
                                    </h3>
                                    {loadingMembers ? (
                                        <div className="text-center py-8 text-white/40">
                                            <p>Loading members...</p>
                                        </div>
                                    ) : members.length === 0 ? (
                                        <div className="text-center py-8 text-white/40">
                                            <Users className="w-8 h-8 mx-auto mb-2 opacity-50" />
                                            <p className="text-sm">No members yet</p>
                                        </div>
                                    ) : (
                                        <div className="space-y-2">
                                            {members.map((member) => (
                                                <div
                                                    key={member.uid}
                                                    className="flex items-center justify-between p-3 bg-white/5 rounded-xl"
                                                >
                                                    <div className="flex items-center gap-3">
                                                        <div className="w-10 h-10 rounded-full bg-gradient-to-br from-blue-500 to-purple-500 flex items-center justify-center">
                                                            <span className="text-white font-medium">
                                                                {(member.name || member.username || '?')[0].toUpperCase()}
                                                            </span>
                                                        </div>
                                                        <div>
                                                            <p className="text-sm font-medium text-white">
                                                                {member.name || member.username}
                                                            </p>
                                                            <p className="text-xs text-white/50">
                                                                @{member.username}
                                                            </p>
                                                        </div>
                                                    </div>
                                                    <div className="flex items-center gap-2">
                                                        {member.role === 'admin' && (
                                                            <span className="flex items-center gap-1 px-2 py-1 bg-yellow-500/20 rounded text-xs text-yellow-400">
                                                                <Crown className="w-3 h-3" />
                                                                Admin
                                                            </span>
                                                        )}
                                                        <button
                                                            onClick={() => handleRemoveMember(member.username)}
                                                            className="p-1.5 hover:bg-red-500/20 rounded transition-colors"
                                                            title="Remove member"
                                                        >
                                                            <X className="w-4 h-4 text-red-400" />
                                                        </button>
                                                    </div>
                                                </div>
                                            ))}
                                        </div>
                                    )}
                                </div>
                            </div>

                            {/* Chat Panel */}
                            {showChat && (
                                <div className="w-1/2 flex flex-col bg-[#0A0A0A]">
                                    {/* Chat Header */}
                                    <div className="p-4 border-b border-white/10">
                                        <div className="flex items-center gap-2">
                                            <MessageCircle className="w-5 h-5 text-green-400" />
                                            <span className="text-sm font-medium text-white">Group Chat</span>
                                            <span className="text-xs text-white/40 ml-2">
                                                Shared memory for all members
                                            </span>
                                        </div>
                                    </div>

                                    {/* Chat Messages */}
                                    <div className="flex-1 overflow-y-auto p-4 space-y-4">
                                        {chatMessages.length === 0 ? (
                                            <div className="h-full flex items-center justify-center">
                                                <div className="text-center text-white/40">
                                                    <MessageCircle className="w-10 h-10 mx-auto mb-3 opacity-50" />
                                                    <p className="text-sm">Start chatting with the group</p>
                                                    <p className="text-xs mt-1">All members share this conversation's memory</p>
                                                </div>
                                            </div>
                                        ) : (
                                            chatMessages.map((msg) => (
                                                <div
                                                    key={msg.id}
                                                    className={cn(
                                                        "flex",
                                                        msg.role === 'user' ? 'justify-end' : 'justify-start'
                                                    )}
                                                >
                                                    <div
                                                        className={cn(
                                                            "max-w-[80%] px-4 py-3 rounded-2xl",
                                                            msg.role === 'user'
                                                                ? 'bg-blue-500 text-white rounded-br-md'
                                                                : 'bg-white/10 text-white rounded-bl-md'
                                                        )}
                                                    >
                                                        <p className="text-sm whitespace-pre-wrap">{msg.content}</p>
                                                    </div>
                                                </div>
                                            ))
                                        )}
                                        {isSending && (
                                            <div className="flex justify-start">
                                                <div className="bg-white/10 px-4 py-3 rounded-2xl rounded-bl-md">
                                                    <div className="flex gap-1">
                                                        <div className="w-2 h-2 rounded-full bg-white/50 animate-bounce" style={{ animationDelay: '0ms' }} />
                                                        <div className="w-2 h-2 rounded-full bg-white/50 animate-bounce" style={{ animationDelay: '150ms' }} />
                                                        <div className="w-2 h-2 rounded-full bg-white/50 animate-bounce" style={{ animationDelay: '300ms' }} />
                                                    </div>
                                                </div>
                                            </div>
                                        )}
                                        <div ref={chatEndRef} />
                                    </div>

                                    {/* Chat Input */}
                                    <div className="p-4 border-t border-white/10">
                                        <div className="flex gap-2">
                                            <input
                                                type="text"
                                                value={chatInput}
                                                onChange={(e) => setChatInput(e.target.value)}
                                                onKeyPress={(e) => e.key === 'Enter' && handleSendMessage()}
                                                placeholder="Type a message..."
                                                className="flex-1 px-4 py-3 bg-white/5 rounded-xl text-white placeholder-white/40 outline-none focus:ring-2 focus:ring-green-500"
                                                disabled={isSending}
                                            />
                                            <button
                                                onClick={handleSendMessage}
                                                disabled={!chatInput.trim() || isSending}
                                                className="px-4 py-3 bg-green-500 hover:bg-green-600 disabled:opacity-50 disabled:cursor-not-allowed rounded-xl text-white transition-colors"
                                            >
                                                <Send className="w-5 h-5" />
                                            </button>
                                        </div>
                                    </div>
                                </div>
                            )}
                        </div>
                    </>
                ) : (
                    <div className="flex-1 flex items-center justify-center">
                        <div className="text-center">
                            <Users className="w-12 h-12 text-white/20 mx-auto mb-4" />
                            <p className="text-white/40">Select a group to view details</p>
                            <button
                                onClick={() => setShowCreateModal(true)}
                                className="mt-4 px-4 py-2 bg-blue-500 hover:bg-blue-600 rounded-lg text-sm text-white transition-colors"
                            >
                                Create New Group
                            </button>
                        </div>
                    </div>
                )}
            </div>

            {/* Create Group Modal */}
            {showCreateModal && (
                <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
                    <div className="bg-[#2C2C2E] rounded-2xl p-6 w-full max-w-md mx-4">
                        <div className="flex items-center justify-between mb-6">
                            <h3 className="text-lg font-semibold text-white">Create New Group</h3>
                            <button
                                onClick={() => setShowCreateModal(false)}
                                className="p-1 hover:bg-white/10 rounded-lg transition-colors"
                            >
                                <X className="w-5 h-5 text-white/60" />
                            </button>
                        </div>
                        <div className="space-y-4">
                            <div>
                                <label className="block text-sm text-white/60 mb-2">Group Name</label>
                                <input
                                    type="text"
                                    value={newGroupName}
                                    onChange={(e) => setNewGroupName(e.target.value)}
                                    placeholder="Enter group name"
                                    className="w-full px-4 py-3 bg-white/5 rounded-xl text-white placeholder-white/40 outline-none focus:ring-2 focus:ring-blue-500"
                                    autoFocus
                                />
                            </div>
                            <div>
                                <label className="block text-sm text-white/60 mb-2">Description</label>
                                <textarea
                                    value={newGroupDesc}
                                    onChange={(e) => setNewGroupDesc(e.target.value)}
                                    placeholder="Enter group description (optional)"
                                    rows={3}
                                    className="w-full px-4 py-3 bg-white/5 rounded-xl text-white placeholder-white/40 outline-none focus:ring-2 focus:ring-blue-500 resize-none"
                                />
                            </div>
                            <div className="flex gap-3 pt-2">
                                <button
                                    onClick={() => setShowCreateModal(false)}
                                    className="flex-1 px-4 py-3 bg-white/10 hover:bg-white/20 rounded-xl text-white transition-colors"
                                >
                                    Cancel
                                </button>
                                <button
                                    onClick={handleCreateGroup}
                                    disabled={!newGroupName.trim() || isSubmitting}
                                    className="flex-1 px-4 py-3 bg-blue-500 hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed rounded-xl text-white transition-colors"
                                >
                                    {isSubmitting ? 'Creating...' : 'Create Group'}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Add Member Modal - Enhanced with User List */}
            {showAddMemberModal && (
                <AddMemberModal
                    isOpen={showAddMemberModal}
                    onClose={() => {
                        setShowAddMemberModal(false);
                        setNewMemberUsername('');
                        setSelectedRole('member');
                        setUserSearchQuery('');
                    }}
                    allUsers={allUsers}
                    loadingUsers={loadingUsers}
                    userSearchQuery={userSearchQuery}
                    setUserSearchQuery={setUserSearchQuery}
                    selectedUsername={newMemberUsername}
                    setSelectedUsername={setNewMemberUsername}
                    selectedRole={selectedRole}
                    setSelectedRole={setSelectedRole}
                    onAddMember={handleAddMember}
                    isSubmitting={isSubmitting}
                    members={members}
                    pendingInvitations={pendingInvitations}
                    groupId={selectedGroup || undefined}
                    onMemberAdded={() => selectedGroup && fetchMembers(selectedGroup)}
                    onFetchUsers={async () => {
                        setLoadingUsers(true);
                        try {
                            const users = await api.getAllUsers();
                            setAllUsers(users);
                        } catch (e) {
                            console.error("Failed to fetch users", e);
                        } finally {
                            setLoadingUsers(false);
                        }
                    }}
                    onFetchInvitations={async () => {
                        // Fetch pending invitations SENT BY this workspace
                        if (!selectedGroup) return;
                        try {
                            const invitations = await api.getWorkspaceSentInvitations(selectedGroup);
                            setPendingInvitations(invitations);
                        } catch (e) {
                            console.error("Failed to fetch workspace invitations", e);
                            setPendingInvitations([]);
                        }
                    }}
                />

            )}

        </div>
    );
};

export default Groups;
