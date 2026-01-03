import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Badge } from '@/components/ui/badge';
import { useToast } from '@/hooks/use-toast';
import {
    ArrowLeft,
    Users,
    Settings,
    Activity,
    Layers,
    RefreshCw,
    Trash2,
    Shield,
    ShieldCheck,
    Loader2,
    BarChart3,
    Zap,
    Database,
    CheckCircle2,
    XCircle,
} from 'lucide-react';

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:3000';

interface AdminUser {
    username: string;
    role: 'admin' | 'user';
}

interface PreCortexMetrics {
    total_requests: number;
    cached_responses: number;
    reflex_responses: number;
    llm_passthrough: number;
    cache_hit_rate: number;
    reflex_rate: number;
    llm_rate: number;
}

interface IngestionMetrics {
    total_processed: number;
    total_errors: number;
    total_entities_created: number;
    success_rate: number;
    avg_duration_ms: number;
    last_processed_at?: string;
}

interface SystemHealthMetrics {
    services_up: Record<string, boolean>;
    service_latency_ms?: Record<string, number>;
}

interface SystemStats {
    total_users: number;
    total_admins: number;
    timestamp: string;
    kernel_stats?: Record<string, unknown>;
    precortex_stats?: PreCortexMetrics;
    ingestion_stats?: IngestionMetrics;
    system_health?: SystemHealthMetrics;
}

interface ActivityEntry {
    timestamp: string;
    user_id: string;
    action: string;
    details: string;
}

interface UserDetails {
    username: string;
    role: string;
    created_at?: string;
    last_active?: string;
    trial_expires_at?: string;
    is_trial: boolean;
    memory_stats?: Record<string, unknown>;
    group_count: number;
}

export default function Admin() {
    const navigate = useNavigate();
    const { user, isAdmin, logout } = useAuth();
    const { toast } = useToast();

    const [users, setUsers] = useState<AdminUser[]>([]);
    const [stats, setStats] = useState<SystemStats | null>(null);
    const [activities, setActivities] = useState<ActivityEntry[]>([]);
    const [isLoadingUsers, setIsLoadingUsers] = useState(true);
    const [isLoadingStats, setIsLoadingStats] = useState(true);
    const [isLoadingActivity, setIsLoadingActivity] = useState(true);
    const [isProcessing, setIsProcessing] = useState<string | null>(null);

    // New state for enhanced user management
    const [searchQuery, setSearchQuery] = useState('');
    const [roleFilter, setRoleFilter] = useState<string>('');
    const [selectedUser, setSelectedUser] = useState<UserDetails | null>(null);
    const [showUserDetails, setShowUserDetails] = useState(false);
    const [trialDays, setTrialDays] = useState(7);
    const [selectedUsernames, setSelectedUsernames] = useState<Set<string>>(new Set());

    // Redirect non-admins
    useEffect(() => {
        if (!isAdmin && user) {
            navigate('/dashboard');
        } else if (!user) {
            navigate('/auth');
        }
    }, [isAdmin, user, navigate]);

    // Fetch users
    useEffect(() => {
        async function fetchUsers() {
            if (!user?.token) return;
            try {
                const response = await fetch(`${API_BASE_URL}/api/admin/users`, {
                    headers: { Authorization: `Bearer ${user.token}` },
                });
                if (response.ok) {
                    const data = await response.json();
                    setUsers(data.users || []);
                }
            } catch (error) {
                console.error('Failed to fetch users:', error);
            } finally {
                setIsLoadingUsers(false);
            }
        }
        fetchUsers();
    }, [user?.token]);

    // Fetch stats
    useEffect(() => {
        async function fetchStats() {
            if (!user?.token) return;
            try {
                const response = await fetch(`${API_BASE_URL}/api/admin/system/stats`, {
                    headers: { Authorization: `Bearer ${user.token}` },
                });
                if (response.ok) {
                    const data = await response.json();
                    setStats(data);
                }
            } catch (error) {
                console.error('Failed to fetch stats:', error);
            } finally {
                setIsLoadingStats(false);
            }
        }
        fetchStats();
    }, [user?.token]);

    // Fetch activity
    useEffect(() => {
        async function fetchActivity() {
            if (!user?.token) return;
            try {
                const response = await fetch(`${API_BASE_URL}/api/admin/activity`, {
                    headers: { Authorization: `Bearer ${user.token}` },
                });
                if (response.ok) {
                    const data = await response.json();
                    setActivities(data.activities || []);
                }
            } catch (error) {
                console.error('Failed to fetch activity:', error);
            } finally {
                setIsLoadingActivity(false);
            }
        }
        fetchActivity();
    }, [user?.token]);

    const handleToggleRole = async (username: string, currentRole: string) => {
        if (!user?.token) return;
        const newRole = currentRole === 'admin' ? 'user' : 'admin';
        setIsProcessing(username);

        try {
            const response = await fetch(`${API_BASE_URL}/api/admin/users/${username}/role`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${user.token}`,
                },
                body: JSON.stringify({ role: newRole }),
            });

            if (response.ok) {
                setUsers(users.map(u =>
                    u.username === username ? { ...u, role: newRole as 'admin' | 'user' } : u
                ));
                toast({
                    title: 'Role updated',
                    description: `${username} is now a ${newRole}`,
                });
            } else {
                const errorText = await response.text();
                toast({
                    variant: 'destructive',
                    title: 'Failed to update role',
                    description: errorText,
                });
            }
        } catch (error) {
            toast({
                variant: 'destructive',
                title: 'Error',
                description: 'Network error',
            });
        } finally {
            setIsProcessing(null);
        }
    };

    const handleDeleteUser = async (username: string) => {
        if (!user?.token) return;
        if (!confirm(`Are you sure you want to delete user "${username}"?`)) return;

        setIsProcessing(username);

        try {
            const response = await fetch(`${API_BASE_URL}/api/admin/users/${username}`, {
                method: 'DELETE',
                headers: { Authorization: `Bearer ${user.token}` },
            });

            if (response.ok) {
                setUsers(users.filter(u => u.username !== username));
                toast({
                    title: 'User deleted',
                    description: `${username} has been removed`,
                });
            } else {
                const errorText = await response.text();
                toast({
                    variant: 'destructive',
                    title: 'Failed to delete user',
                    description: errorText,
                });
            }
        } catch (error) {
            toast({
                variant: 'destructive',
                title: 'Error',
                description: 'Network error',
            });
        } finally {
            setIsProcessing(null);
        }
    };

    const handleTriggerReflection = async () => {
        if (!user?.token) return;
        setIsProcessing('reflection');

        try {
            const response = await fetch(`${API_BASE_URL}/api/admin/system/reflection`, {
                method: 'POST',
                headers: { Authorization: `Bearer ${user.token}` },
            });

            if (response.ok) {
                toast({
                    title: 'Reflection triggered',
                    description: 'The memory kernel is processing a reflection cycle',
                });
            } else {
                toast({
                    variant: 'destructive',
                    title: 'Failed to trigger reflection',
                });
            }
        } catch (error) {
            toast({
                variant: 'destructive',
                title: 'Error',
                description: 'Network error',
            });
        } finally {
            setIsProcessing(null);
        }
    };

    const viewUserDetails = async (username: string) => {
        if (!user?.token) return;
        setIsProcessing(username);

        try {
            const response = await fetch(`${API_BASE_URL}/api/admin/users/${username}/details`, {
                headers: { Authorization: `Bearer ${user.token}` },
            });

            if (response.ok) {
                const data = await response.json();
                setSelectedUser(data);
                setShowUserDetails(true);
            } else {
                toast({
                    variant: 'destructive',
                    title: 'Failed to load user details',
                });
            }
        } catch (error) {
            toast({
                variant: 'destructive',
                title: 'Error',
                description: 'Network error',
            });
        } finally {
            setIsProcessing(null);
        }
    };

    const handleExtendTrial = async (username: string, days: number) => {
        if (!user?.token) return;
        setIsProcessing('trial-' + username);

        try {
            const response = await fetch(`${API_BASE_URL}/api/admin/users/${username}/trial`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${user.token}`,
                },
                body: JSON.stringify({ days }),
            });

            if (response.ok) {
                const data = await response.json();
                toast({
                    title: 'Trial extended',
                    description: `${username}'s trial extended until ${new Date(data.expires_at).toLocaleDateString()}`,
                });
                // Refresh user details
                if (selectedUser?.username === username) {
                    viewUserDetails(username);
                }
            } else {
                toast({
                    variant: 'destructive',
                    title: 'Failed to extend trial',
                });
            }
        } catch (error) {
            toast({
                variant: 'destructive',
                title: 'Error',
                description: 'Network error',
            });
        } finally {
            setIsProcessing(null);
        }
    };

    // Filter users based on search and role
    const filteredUsers = users.filter(u => {
        const matchesSearch = searchQuery === '' || u.username.toLowerCase().includes(searchQuery.toLowerCase());
        const matchesRole = roleFilter === '' || u.role === roleFilter;
        return matchesSearch && matchesRole;
    });

    // Toggle user selection
    const toggleUserSelection = (username: string) => {
        const newSet = new Set(selectedUsernames);
        if (newSet.has(username)) {
            newSet.delete(username);
        } else {
            newSet.add(username);
        }
        setSelectedUsernames(newSet);
    };

    // Select/deselect all filtered users
    const toggleSelectAll = () => {
        if (selectedUsernames.size === filteredUsers.length) {
            setSelectedUsernames(new Set());
        } else {
            setSelectedUsernames(new Set(filteredUsers.map(u => u.username)));
        }
    };

    // Export users
    const handleExportUsers = (format: 'json' | 'csv') => {
        window.open(`${API_BASE_URL}/api/admin/export/users?format=${format}`, '_blank');
    };

    // Batch role update
    const handleBatchRole = async (role: 'admin' | 'user') => {
        if (!user?.token || selectedUsernames.size === 0) return;
        setIsProcessing('batch-role');

        try {
            const response = await fetch(`${API_BASE_URL}/api/admin/batch/role`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${user.token}`,
                },
                body: JSON.stringify({ usernames: Array.from(selectedUsernames), role }),
            });

            if (response.ok) {
                const data = await response.json();
                toast({
                    title: 'Batch update complete',
                    description: `Updated ${data.updated} users to ${role}`,
                });
                // Refresh users
                setSelectedUsernames(new Set());
                window.location.reload();
            }
        } catch (error) {
            toast({ variant: 'destructive', title: 'Batch update failed' });
        } finally {
            setIsProcessing(null);
        }
    };

    // Batch delete
    const handleBatchDelete = async () => {
        if (!user?.token || selectedUsernames.size === 0) return;
        if (!confirm(`Delete ${selectedUsernames.size} users? This cannot be undone.`)) return;

        setIsProcessing('batch-delete');

        try {
            const response = await fetch(`${API_BASE_URL}/api/admin/batch/delete`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${user.token}`,
                },
                body: JSON.stringify({ usernames: Array.from(selectedUsernames) }),
            });

            if (response.ok) {
                const data = await response.json();
                toast({
                    title: 'Batch delete complete',
                    description: `Deleted ${data.deleted} users`,
                });
                setSelectedUsernames(new Set());
                window.location.reload();
            }
        } catch (error) {
            toast({ variant: 'destructive', title: 'Batch delete failed' });
        } finally {
            setIsProcessing(null);
        }
    };

    if (!isAdmin) {
        return null;
    }

    return (
        <div className="min-h-screen bg-background">
            {/* Header */}
            <header className="border-b border-border/50 bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 sticky top-0 z-50">
                <div className="container mx-auto px-4 py-4 flex items-center justify-between">
                    <div className="flex items-center gap-4">
                        <button
                            onClick={() => navigate('/dashboard')}
                            className="flex items-center gap-2 text-muted-foreground hover:text-foreground transition-colors text-sm"
                        >
                            <ArrowLeft className="w-4 h-4" />
                            Back to Dashboard
                        </button>
                        <div className="flex items-center gap-2">
                            <div className="w-8 h-8 bg-primary rounded-lg flex items-center justify-center">
                                <ShieldCheck className="w-4 h-4 text-primary-foreground" />
                            </div>
                            <h1 className="text-xl font-semibold">Admin Panel</h1>
                        </div>
                    </div>
                    <div className="flex items-center gap-4">
                        <Badge variant="outline" className="gap-1">
                            <Shield className="w-3 h-3" />
                            {user?.username}
                        </Badge>
                        <Button variant="ghost" size="sm" onClick={logout}>
                            Logout
                        </Button>
                    </div>
                </div>
            </header>

            {/* Main Content */}
            <main className="container mx-auto px-4 py-8">
                <motion.div
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.5 }}
                >
                    <Tabs defaultValue="users" className="space-y-6">
                        <TabsList className="grid w-full grid-cols-5 max-w-2xl">
                            <TabsTrigger value="users" className="gap-2">
                                <Users className="w-4 h-4" />
                                Users
                            </TabsTrigger>
                            <TabsTrigger value="system" className="gap-2">
                                <Settings className="w-4 h-4" />
                                System
                            </TabsTrigger>
                            <TabsTrigger value="groups" className="gap-2">
                                <Layers className="w-4 h-4" />
                                Groups
                            </TabsTrigger>
                            <TabsTrigger value="activity" className="gap-2">
                                <Activity className="w-4 h-4" />
                                Activity
                            </TabsTrigger>
                            <TabsTrigger value="metrics" className="gap-2">
                                <BarChart3 className="w-4 h-4" />
                                Metrics
                            </TabsTrigger>
                        </TabsList>

                        {/* Users Tab */}
                        <TabsContent value="users">
                            <Card>
                                <CardHeader>
                                    <CardTitle>User Management</CardTitle>
                                    <CardDescription>
                                        Manage user accounts and roles. Admins have access to this panel.
                                    </CardDescription>
                                </CardHeader>
                                <CardContent>
                                    {/* Search and Filter Bar */}
                                    <div className="flex flex-col sm:flex-row gap-3 mb-6">
                                        <div className="flex-1 relative">
                                            <input
                                                type="text"
                                                placeholder="Search users..."
                                                value={searchQuery}
                                                onChange={(e) => setSearchQuery(e.target.value)}
                                                className="w-full px-4 py-2 rounded-lg border border-border bg-muted/30 focus:outline-none focus:ring-2 focus:ring-primary/50"
                                            />
                                        </div>
                                        <select
                                            value={roleFilter}
                                            onChange={(e) => setRoleFilter(e.target.value)}
                                            className="px-4 py-2 rounded-lg border border-border bg-muted/30 focus:outline-none focus:ring-2 focus:ring-primary/50"
                                        >
                                            <option value="">All Roles</option>
                                            <option value="admin">Admins Only</option>
                                            <option value="user">Users Only</option>
                                        </select>
                                    </div>

                                    {/* Action Bar */}
                                    <div className="flex flex-wrap items-center gap-3 mb-4">
                                        <p className="text-sm text-muted-foreground">
                                            Showing {filteredUsers.length} of {users.length} users
                                        </p>
                                        <div className="flex-1" />
                                        <Button variant="outline" size="sm" onClick={() => handleExportUsers('csv')}>
                                            Export CSV
                                        </Button>
                                        <Button variant="outline" size="sm" onClick={() => handleExportUsers('json')}>
                                            Export JSON
                                        </Button>
                                    </div>

                                    {/* Batch Actions Bar (shown when users selected) */}
                                    {selectedUsernames.size > 0 && (
                                        <div className="flex items-center gap-3 p-3 mb-4 rounded-lg bg-primary/10 border border-primary/20">
                                            <span className="text-sm font-medium">{selectedUsernames.size} selected</span>
                                            <div className="flex-1" />
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => handleBatchRole('admin')}
                                                disabled={isProcessing === 'batch-role'}
                                            >
                                                Make Admin
                                            </Button>
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => handleBatchRole('user')}
                                                disabled={isProcessing === 'batch-role'}
                                            >
                                                Make User
                                            </Button>
                                            <Button
                                                variant="destructive"
                                                size="sm"
                                                onClick={handleBatchDelete}
                                                disabled={isProcessing === 'batch-delete'}
                                            >
                                                Delete Selected
                                            </Button>
                                        </div>
                                    )}

                                    {isLoadingUsers ? (
                                        <div className="flex justify-center py-8">
                                            <Loader2 className="w-6 h-6 animate-spin text-primary" />
                                        </div>
                                    ) : filteredUsers.length === 0 ? (
                                        <p className="text-muted-foreground text-center py-8">No users found</p>
                                    ) : (
                                        <div className="space-y-3">
                                            {filteredUsers.map((u) => (
                                                <div
                                                    key={u.username}
                                                    className={`flex items-center justify-between p-4 rounded-lg border ${selectedUsernames.has(u.username) ? 'border-primary bg-primary/5' : 'border-border/50 bg-muted/30'}`}
                                                >
                                                    <div className="flex items-center gap-3">
                                                        <input
                                                            type="checkbox"
                                                            checked={selectedUsernames.has(u.username)}
                                                            onChange={() => toggleUserSelection(u.username)}
                                                            className="w-4 h-4 rounded border-border"
                                                        />
                                                        <div className="w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center">
                                                            {u.role === 'admin' ? (
                                                                <ShieldCheck className="w-5 h-5 text-primary" />
                                                            ) : (
                                                                <Users className="w-5 h-5 text-muted-foreground" />
                                                            )}
                                                        </div>
                                                        <div>
                                                            <p className="font-medium">{u.username}</p>
                                                            <span className={`text-xs px-2 py-0.5 rounded-full ${u.role === 'admin' ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'}`}>
                                                                {u.role}
                                                            </span>
                                                        </div>
                                                    </div>
                                                    <div className="flex items-center gap-2">
                                                        <Button
                                                            variant="ghost"
                                                            size="sm"
                                                            onClick={() => viewUserDetails(u.username)}
                                                            disabled={isProcessing === u.username}
                                                        >
                                                            View
                                                        </Button>
                                                        {u.username !== user?.username && (
                                                            <>
                                                                <Button
                                                                    variant="outline"
                                                                    size="sm"
                                                                    onClick={() => handleToggleRole(u.username, u.role)}
                                                                    disabled={isProcessing === u.username}
                                                                >
                                                                    {isProcessing === u.username ? (
                                                                        <Loader2 className="w-4 h-4 animate-spin" />
                                                                    ) : u.role === 'admin' ? (
                                                                        'Demote'
                                                                    ) : (
                                                                        'Promote'
                                                                    )}
                                                                </Button>
                                                                <Button
                                                                    variant="destructive"
                                                                    size="sm"
                                                                    onClick={() => handleDeleteUser(u.username)}
                                                                    disabled={isProcessing === u.username}
                                                                >
                                                                    <Trash2 className="w-4 h-4" />
                                                                </Button>
                                                            </>
                                                        )}
                                                    </div>
                                                </div>
                                            ))}
                                        </div>
                                    )}

                                    {/* User Details Modal */}
                                    {showUserDetails && selectedUser && (
                                        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowUserDetails(false)}>
                                            <div className="bg-background rounded-xl p-6 max-w-md w-full mx-4 border border-border shadow-2xl" onClick={(e) => e.stopPropagation()}>
                                                <div className="flex items-center justify-between mb-4">
                                                    <h3 className="text-lg font-semibold">User Details</h3>
                                                    <button onClick={() => setShowUserDetails(false)} className="text-muted-foreground hover:text-foreground">×</button>
                                                </div>
                                                <div className="space-y-4">
                                                    <div className="flex items-center gap-3">
                                                        <div className="w-12 h-12 rounded-full bg-primary/10 flex items-center justify-center">
                                                            <Users className="w-6 h-6 text-primary" />
                                                        </div>
                                                        <div>
                                                            <p className="font-medium text-lg">{selectedUser.username}</p>
                                                            <span className={`text-xs px-2 py-0.5 rounded-full ${selectedUser.role === 'admin' ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'}`}>
                                                                {selectedUser.role}
                                                            </span>
                                                        </div>
                                                    </div>

                                                    <div className="grid grid-cols-2 gap-3">
                                                        <div className="p-3 rounded-lg bg-muted/50">
                                                            <p className="text-xs text-muted-foreground">Groups</p>
                                                            <p className="font-medium">{selectedUser.group_count}</p>
                                                        </div>
                                                        <div className="p-3 rounded-lg bg-muted/50">
                                                            <p className="text-xs text-muted-foreground">Trial</p>
                                                            <p className="font-medium">{selectedUser.is_trial ? 'Yes' : 'No'}</p>
                                                        </div>
                                                    </div>

                                                    {selectedUser.trial_expires_at && (
                                                        <div className="p-3 rounded-lg bg-amber-500/10 border border-amber-500/20">
                                                            <p className="text-xs text-amber-500">Trial Expires</p>
                                                            <p className="font-medium text-amber-500">
                                                                {new Date(selectedUser.trial_expires_at).toLocaleDateString()}
                                                            </p>
                                                        </div>
                                                    )}

                                                    {/* Trial Extension */}
                                                    <div className="pt-4 border-t border-border">
                                                        <p className="text-sm font-medium mb-2">Extend Trial</p>
                                                        <div className="flex gap-2">
                                                            <select
                                                                value={trialDays}
                                                                onChange={(e) => setTrialDays(Number(e.target.value))}
                                                                className="flex-1 px-3 py-2 rounded-lg border border-border bg-muted/30"
                                                            >
                                                                <option value={7}>7 days</option>
                                                                <option value={14}>14 days</option>
                                                                <option value={30}>30 days</option>
                                                                <option value={90}>90 days</option>
                                                            </select>
                                                            <Button
                                                                onClick={() => handleExtendTrial(selectedUser.username, trialDays)}
                                                                disabled={isProcessing?.startsWith('trial-')}
                                                            >
                                                                {isProcessing?.startsWith('trial-') ? (
                                                                    <Loader2 className="w-4 h-4 animate-spin" />
                                                                ) : (
                                                                    'Extend'
                                                                )}
                                                            </Button>
                                                        </div>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    )}
                                </CardContent>
                            </Card>
                        </TabsContent>

                        {/* System Tab */}
                        <TabsContent value="system">
                            <div className="grid gap-6 md:grid-cols-2">
                                <Card>
                                    <CardHeader>
                                        <CardTitle>System Statistics</CardTitle>
                                        <CardDescription>Current system health and metrics</CardDescription>
                                    </CardHeader>
                                    <CardContent>
                                        {isLoadingStats ? (
                                            <div className="flex justify-center py-8">
                                                <Loader2 className="w-6 h-6 animate-spin text-primary" />
                                            </div>
                                        ) : stats ? (
                                            <div className="space-y-4">
                                                <div className="grid grid-cols-2 gap-4">
                                                    <div className="p-4 rounded-lg bg-muted/50">
                                                        <p className="text-sm text-muted-foreground">Total Users</p>
                                                        <p className="text-2xl font-bold">{stats.total_users}</p>
                                                    </div>
                                                    <div className="p-4 rounded-lg bg-muted/50">
                                                        <p className="text-sm text-muted-foreground">Admins</p>
                                                        <p className="text-2xl font-bold">{stats.total_admins}</p>
                                                    </div>
                                                </div>
                                                <p className="text-xs text-muted-foreground">
                                                    Last updated: {new Date(stats.timestamp).toLocaleString()}
                                                </p>
                                            </div>
                                        ) : (
                                            <p className="text-muted-foreground">No stats available</p>
                                        )}
                                    </CardContent>
                                </Card>

                                <Card>
                                    <CardHeader>
                                        <CardTitle>Memory Kernel Controls</CardTitle>
                                        <CardDescription>Manage the reflection engine</CardDescription>
                                    </CardHeader>
                                    <CardContent className="space-y-4">
                                        <Button
                                            onClick={handleTriggerReflection}
                                            disabled={isProcessing === 'reflection'}
                                            className="w-full"
                                        >
                                            {isProcessing === 'reflection' ? (
                                                <Loader2 className="w-4 h-4 animate-spin mr-2" />
                                            ) : (
                                                <RefreshCw className="w-4 h-4 mr-2" />
                                            )}
                                            Trigger Reflection Cycle
                                        </Button>
                                        <p className="text-xs text-muted-foreground">
                                            Manually triggers the memory kernel's reflection process to discover insights and patterns.
                                        </p>
                                    </CardContent>
                                </Card>

                                {/* Cache Management */}
                                <Card>
                                    <CardHeader>
                                        <CardTitle>Cache Management</CardTitle>
                                        <CardDescription>Clear cached data to free memory</CardDescription>
                                    </CardHeader>
                                    <CardContent className="space-y-4">
                                        <div className="grid grid-cols-2 gap-2">
                                            <Button
                                                variant="outline"
                                                onClick={async () => {
                                                    if (!user?.token) return;
                                                    setIsProcessing('cache-sessions');
                                                    try {
                                                        const res = await fetch(`${API_BASE_URL}/api/admin/system/cache/clear`, {
                                                            method: 'POST',
                                                            headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${user.token}` },
                                                            body: JSON.stringify({ type: 'sessions' }),
                                                        });
                                                        if (res.ok) {
                                                            const data = await res.json();
                                                            toast({ title: 'Cache cleared', description: `Cleared ${data.cleared} session keys` });
                                                        }
                                                    } catch {
                                                        toast({ variant: 'destructive', title: 'Failed to clear cache' });
                                                    } finally {
                                                        setIsProcessing(null);
                                                    }
                                                }}
                                                disabled={isProcessing?.startsWith('cache-')}
                                            >
                                                {isProcessing === 'cache-sessions' ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null}
                                                Clear Sessions
                                            </Button>
                                            <Button
                                                variant="outline"
                                                onClick={async () => {
                                                    if (!user?.token) return;
                                                    setIsProcessing('cache-precortex');
                                                    try {
                                                        const res = await fetch(`${API_BASE_URL}/api/admin/system/cache/clear`, {
                                                            method: 'POST',
                                                            headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${user.token}` },
                                                            body: JSON.stringify({ type: 'precortex' }),
                                                        });
                                                        if (res.ok) {
                                                            const data = await res.json();
                                                            toast({ title: 'Cache cleared', description: `Cleared ${data.cleared} Pre-Cortex keys` });
                                                        }
                                                    } catch {
                                                        toast({ variant: 'destructive', title: 'Failed to clear cache' });
                                                    } finally {
                                                        setIsProcessing(null);
                                                    }
                                                }}
                                                disabled={isProcessing?.startsWith('cache-')}
                                            >
                                                {isProcessing === 'cache-precortex' ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null}
                                                Clear Pre-Cortex
                                            </Button>
                                        </div>
                                        <p className="text-xs text-muted-foreground">
                                            Clearing cache may temporarily slow down responses while caches rebuild.
                                        </p>
                                    </CardContent>
                                </Card>
                            </div>
                        </TabsContent>

                        {/* Groups Tab */}
                        <TabsContent value="groups">
                            <Card>
                                <CardHeader>
                                    <CardTitle>Groups Overview</CardTitle>
                                    <CardDescription>All groups in the system with member counts</CardDescription>
                                </CardHeader>
                                <CardContent>
                                    {isLoadingStats ? (
                                        <div className="flex justify-center py-8">
                                            <Loader2 className="w-6 h-6 animate-spin text-primary" />
                                        </div>
                                    ) : (
                                        <div className="space-y-3">
                                            {/* Groups will be loaded from stats.groups if available */}
                                            <div className="grid gap-4 md:grid-cols-2">
                                                {/* Group stats card */}
                                                <div className="p-4 rounded-lg border border-border/50 bg-muted/30">
                                                    <div className="flex items-center gap-3">
                                                        <div className="w-12 h-12 rounded-full bg-blue-500/10 flex items-center justify-center">
                                                            <Layers className="w-6 h-6 text-blue-500" />
                                                        </div>
                                                        <div>
                                                            <p className="text-2xl font-bold">{stats?.kernel_stats?.total_groups || 0}</p>
                                                            <p className="text-sm text-muted-foreground">Total Groups</p>
                                                        </div>
                                                    </div>
                                                </div>

                                                {/* Active groups card */}
                                                <div className="p-4 rounded-lg border border-border/50 bg-muted/30">
                                                    <div className="flex items-center gap-3">
                                                        <div className="w-12 h-12 rounded-full bg-green-500/10 flex items-center justify-center">
                                                            <Activity className="w-6 h-6 text-green-500" />
                                                        </div>
                                                        <div>
                                                            <p className="text-2xl font-bold">{stats?.kernel_stats?.active_groups || 0}</p>
                                                            <p className="text-sm text-muted-foreground">Active Groups</p>
                                                        </div>
                                                    </div>
                                                </div>
                                            </div>

                                            {/* Info message */}
                                            <div className="p-4 rounded-lg bg-blue-500/10 border border-blue-500/20 mt-4">
                                                <p className="text-sm text-blue-400">
                                                    Groups are managed through the main Groups interface. Use <strong>/groups</strong> to create and manage groups with their members.
                                                </p>
                                            </div>

                                            {/* Quick actions */}
                                            <div className="flex gap-3 mt-4">
                                                <Button variant="outline" onClick={() => window.open('/groups', '_blank')}>
                                                    <Layers className="w-4 h-4 mr-2" />
                                                    Open Groups Panel
                                                </Button>
                                            </div>
                                        </div>
                                    )}
                                </CardContent>
                            </Card>
                        </TabsContent>

                        {/* Activity Tab */}
                        <TabsContent value="activity">
                            <Card>
                                <CardHeader>
                                    <CardTitle>Admin Activity Log</CardTitle>
                                    <CardDescription>Recent administrative actions</CardDescription>
                                </CardHeader>
                                <CardContent>
                                    {/* Activity Filters */}
                                    <div className="flex flex-wrap gap-3 mb-4">
                                        <select
                                            className="px-3 py-2 rounded-lg border border-border bg-muted/30 text-sm"
                                            onChange={(e) => {
                                                // Filter activities locally
                                                const filter = e.target.value;
                                                if (filter === '') {
                                                    // Reset would need re-fetch
                                                }
                                            }}
                                        >
                                            <option value="">All Actions</option>
                                            <option value="role_update">Role Updates</option>
                                            <option value="user_delete">User Deletions</option>
                                            <option value="trial_extend">Trial Extensions</option>
                                            <option value="reflection_trigger">Reflection Triggers</option>
                                        </select>
                                        <span className="text-sm text-muted-foreground py-2">
                                            {activities.length} activities
                                        </span>
                                    </div>

                                    {isLoadingActivity ? (
                                        <div className="flex justify-center py-8">
                                            <Loader2 className="w-6 h-6 animate-spin text-primary" />
                                        </div>
                                    ) : activities.length === 0 ? (
                                        <p className="text-muted-foreground text-center py-8">No recent activity</p>
                                    ) : (
                                        <div className="space-y-3">
                                            {activities.map((activity, index) => (
                                                <div
                                                    key={index}
                                                    className="flex items-start gap-3 p-3 rounded-lg border border-border/50 hover:bg-muted/30 transition-colors"
                                                >
                                                    <div className={`w-8 h-8 rounded-full flex items-center justify-center ${activity.action === 'user_delete' ? 'bg-red-500/10 text-red-500' :
                                                        activity.action === 'role_update' ? 'bg-blue-500/10 text-blue-500' :
                                                            activity.action === 'trial_extend' ? 'bg-amber-500/10 text-amber-500' :
                                                                'bg-primary/10 text-primary'
                                                        }`}>
                                                        <Activity className="w-4 h-4" />
                                                    </div>
                                                    <div className="flex-1">
                                                        <div className="flex items-center gap-2 flex-wrap">
                                                            <span className="font-medium">{activity.user_id}</span>
                                                            <span className={`text-xs px-2 py-0.5 rounded-full ${activity.action === 'user_delete' ? 'bg-red-500/10 text-red-500' :
                                                                activity.action === 'role_update' ? 'bg-blue-500/10 text-blue-500' :
                                                                    activity.action === 'trial_extend' ? 'bg-amber-500/10 text-amber-500' :
                                                                        'bg-muted text-muted-foreground'
                                                                }`}>
                                                                {activity.action.replace('_', ' ')}
                                                            </span>
                                                        </div>
                                                        <p className="text-sm text-muted-foreground">{activity.details}</p>
                                                        <p className="text-xs text-muted-foreground mt-1">
                                                            {new Date(activity.timestamp).toLocaleString()}
                                                        </p>
                                                    </div>
                                                </div>
                                            ))}
                                        </div>
                                    )}
                                </CardContent>
                            </Card>
                        </TabsContent>

                        {/* Metrics Tab */}
                        <TabsContent value="metrics">
                            <div className="grid gap-6 md:grid-cols-2">
                                {/* Pre-Cortex Efficiency */}
                                <Card>
                                    <CardHeader>
                                        <div className="flex items-center gap-2">
                                            <Zap className="w-5 h-5 text-amber-500" />
                                            <CardTitle>Pre-Cortex Efficiency</CardTitle>
                                        </div>
                                        <CardDescription>Semantic caching and cost reduction metrics</CardDescription>
                                    </CardHeader>
                                    <CardContent>
                                        {isLoadingStats ? (
                                            <div className="flex justify-center py-8">
                                                <Loader2 className="w-6 h-6 animate-spin text-primary" />
                                            </div>
                                        ) : stats?.precortex_stats ? (
                                            <div className="space-y-4">
                                                <div className="grid grid-cols-2 gap-4">
                                                    <div className="p-4 rounded-lg bg-gradient-to-br from-green-500/10 to-green-500/5 border border-green-500/20">
                                                        <p className="text-sm text-muted-foreground">Cache Hit Rate</p>
                                                        <p className="text-2xl font-bold text-green-500">
                                                            {stats.precortex_stats.cache_hit_rate.toFixed(1)}%
                                                        </p>
                                                    </div>
                                                    <div className="p-4 rounded-lg bg-gradient-to-br from-blue-500/10 to-blue-500/5 border border-blue-500/20">
                                                        <p className="text-sm text-muted-foreground">Reflex Rate</p>
                                                        <p className="text-2xl font-bold text-blue-500">
                                                            {stats.precortex_stats.reflex_rate.toFixed(1)}%
                                                        </p>
                                                    </div>
                                                </div>
                                                <div className="grid grid-cols-2 gap-4">
                                                    <div className="p-4 rounded-lg bg-muted/50">
                                                        <p className="text-sm text-muted-foreground">LLM Passthrough</p>
                                                        <p className="text-2xl font-bold">
                                                            {stats.precortex_stats.llm_rate.toFixed(1)}%
                                                        </p>
                                                    </div>
                                                    <div className="p-4 rounded-lg bg-muted/50">
                                                        <p className="text-sm text-muted-foreground">Total Requests</p>
                                                        <p className="text-2xl font-bold">
                                                            {stats.precortex_stats.total_requests.toLocaleString()}
                                                        </p>
                                                    </div>
                                                </div>
                                                <div className="p-4 rounded-lg bg-gradient-to-r from-purple-500/10 to-cyan-500/10 border border-purple-500/20">
                                                    <p className="text-sm text-muted-foreground mb-2">Cost Efficiency Breakdown</p>
                                                    <div className="w-full h-4 bg-muted rounded-full overflow-hidden flex">
                                                        <div
                                                            className="h-full bg-green-500"
                                                            style={{ width: `${stats.precortex_stats.cache_hit_rate}%` }}
                                                            title="Cached"
                                                        />
                                                        <div
                                                            className="h-full bg-blue-500"
                                                            style={{ width: `${stats.precortex_stats.reflex_rate}%` }}
                                                            title="Reflex"
                                                        />
                                                        <div
                                                            className="h-full bg-amber-500"
                                                            style={{ width: `${stats.precortex_stats.llm_rate}%` }}
                                                            title="LLM"
                                                        />
                                                    </div>
                                                    <div className="flex justify-between text-xs mt-2 text-muted-foreground">
                                                        <span className="flex items-center gap-1">
                                                            <div className="w-2 h-2 rounded-full bg-green-500" /> Cached
                                                        </span>
                                                        <span className="flex items-center gap-1">
                                                            <div className="w-2 h-2 rounded-full bg-blue-500" /> Reflex
                                                        </span>
                                                        <span className="flex items-center gap-1">
                                                            <div className="w-2 h-2 rounded-full bg-amber-500" /> LLM
                                                        </span>
                                                    </div>
                                                </div>
                                            </div>
                                        ) : (
                                            <p className="text-muted-foreground text-center py-8">
                                                Pre-Cortex not enabled or no data available
                                            </p>
                                        )}
                                    </CardContent>
                                </Card>

                                {/* System Health */}
                                <Card>
                                    <CardHeader>
                                        <div className="flex items-center gap-2">
                                            <Database className="w-5 h-5 text-cyan-500" />
                                            <CardTitle>System Health</CardTitle>
                                        </div>
                                        <CardDescription>Service connectivity status</CardDescription>
                                    </CardHeader>
                                    <CardContent>
                                        {isLoadingStats ? (
                                            <div className="flex justify-center py-8">
                                                <Loader2 className="w-6 h-6 animate-spin text-primary" />
                                            </div>
                                        ) : stats?.system_health ? (
                                            <div className="space-y-3">
                                                {Object.entries(stats.system_health.services_up).map(([service, isUp]) => (
                                                    <div
                                                        key={service}
                                                        className="flex items-center justify-between p-3 rounded-lg border border-border/50"
                                                    >
                                                        <span className="font-medium capitalize">{service.replace('_', ' ')}</span>
                                                        {isUp ? (
                                                            <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-500/10 text-green-500 border border-green-500/20">
                                                                <CheckCircle2 className="w-3 h-3" />
                                                                Healthy
                                                            </span>
                                                        ) : (
                                                            <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-500/10 text-red-500 border border-red-500/20">
                                                                <XCircle className="w-3 h-3" />
                                                                Down
                                                            </span>
                                                        )}
                                                    </div>
                                                ))}
                                            </div>
                                        ) : (
                                            <p className="text-muted-foreground text-center py-8">
                                                No health data available
                                            </p>
                                        )}
                                    </CardContent>
                                </Card>

                                {/* Ingestion Pipeline */}
                                <Card className="md:col-span-2">
                                    <CardHeader>
                                        <div className="flex items-center gap-2">
                                            <BarChart3 className="w-5 h-5 text-purple-500" />
                                            <CardTitle>Ingestion Pipeline</CardTitle>
                                        </div>
                                        <CardDescription>Document processing metrics</CardDescription>
                                    </CardHeader>
                                    <CardContent>
                                        {isLoadingStats ? (
                                            <div className="flex justify-center py-8">
                                                <Loader2 className="w-6 h-6 animate-spin text-primary" />
                                            </div>
                                        ) : stats?.ingestion_stats ? (
                                            <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
                                                <div className="p-4 rounded-lg bg-muted/50 text-center">
                                                    <p className="text-sm text-muted-foreground">Total Processed</p>
                                                    <p className="text-2xl font-bold">
                                                        {stats.ingestion_stats.total_processed.toLocaleString()}
                                                    </p>
                                                </div>
                                                <div className="p-4 rounded-lg bg-muted/50 text-center">
                                                    <p className="text-sm text-muted-foreground">Errors</p>
                                                    <p className="text-2xl font-bold text-red-500">
                                                        {stats.ingestion_stats.total_errors}
                                                    </p>
                                                </div>
                                                <div className="p-4 rounded-lg bg-muted/50 text-center">
                                                    <p className="text-sm text-muted-foreground">Success Rate</p>
                                                    <p className="text-2xl font-bold text-green-500">
                                                        {stats.ingestion_stats.success_rate.toFixed(1)}%
                                                    </p>
                                                </div>
                                                <div className="p-4 rounded-lg bg-muted/50 text-center">
                                                    <p className="text-sm text-muted-foreground">Entities Created</p>
                                                    <p className="text-2xl font-bold">
                                                        {stats.ingestion_stats.total_entities_created.toLocaleString()}
                                                    </p>
                                                </div>
                                                <div className="p-4 rounded-lg bg-muted/50 text-center">
                                                    <p className="text-sm text-muted-foreground">Avg Duration</p>
                                                    <p className="text-2xl font-bold">
                                                        {stats.ingestion_stats.avg_duration_ms.toFixed(0)}ms
                                                    </p>
                                                </div>
                                            </div>
                                        ) : (
                                            <p className="text-muted-foreground text-center py-8">
                                                No ingestion data available
                                            </p>
                                        )}
                                    </CardContent>
                                </Card>
                            </div>
                        </TabsContent>
                    </Tabs>
                </motion.div>
            </main>
        </div>
    );
}
