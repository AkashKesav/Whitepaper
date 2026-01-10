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
    LayoutDashboard,
    Users,
    Activity,
    Shield,
    ShieldCheck,
    Loader2,
    BarChart3,
    Layers,
    RefreshCw,
    Trash2,
    Zap,
    Database,
    CheckCircle2,
    XCircle,
    DollarSign,
    LifeBuoy,
    Share2,
    Megaphone,
    Settings,
    AlertTriangle,
    Lock,
    Plus,
    X,
} from 'lucide-react';

import { api } from '@/lib/api';

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

interface Policy {
    id: string;
    description: string;
    subjects: string[];
    resources: string[];
    actions: string[];
    effect: 'ALLOW' | 'DENY';
    conditions?: Record<string, string>;
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

    // Phase 2 State
    const [revenue, setRevenue] = useState<any>(null);
    const [tickets, setTickets] = useState<any[]>([]);
    const [affiliates, setAffiliates] = useState<any[]>([]);
    const [campaigns, setCampaigns] = useState<any[]>([]);
    const [flags, setFlags] = useState<any[]>([]);
    const [emergencyRequests, setEmergencyRequests] = useState<any[]>([]);

    // Policy Management State
    const [policies, setPolicies] = useState<Policy[]>([]);
    const [isLoadingPolicies, setIsLoadingPolicies] = useState(true);
    const [showCreatePolicy, setShowCreatePolicy] = useState(false);
    const [newPolicy, setNewPolicy] = useState<Policy>({
        id: '',
        description: '',
        subjects: [],
        resources: [],
        actions: [],
        effect: 'DENY',
    });
    const [subjectInput, setSubjectInput] = useState('');
    const [resourceInput, setResourceInput] = useState('');

    // Create User State
    const [showCreateUser, setShowCreateUser] = useState(false);
    const [newUser, setNewUser] = useState({ username: '', password: '', role: 'user' });

    // Create Affiliate State
    const [showCreateAffiliate, setShowCreateAffiliate] = useState(false);
    const [newAffiliate, setNewAffiliate] = useState({ code: '', user: '', commission_rate: 0.1 });

    // Create Campaign State
    const [showCreateCampaign, setShowCreateCampaign] = useState(false);
    const [newCampaign, setNewCampaign] = useState({ id: '', name: '', type: 'email', target_audience: 'all', status: 'draft' });

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
            try {
                const data = await api.admin.getUsers();
                setUsers(data);
            } catch (error) {
                console.error('Failed to fetch users:', error);
                toast({ variant: 'destructive', title: 'Error', description: 'Failed to load users' });
            } finally {
                setIsLoadingUsers(false);
            }
        }
        fetchUsers();
    }, [user?.token, toast]);

    // Fetch stats
    useEffect(() => {
        async function fetchStats() {
            try {
                const data = await api.admin.getStats();
                setStats(data);
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
            try {
                const data = await api.admin.getActivityLog();
                setActivities(data);
            } catch (error) {
                console.error('Failed to fetch activity:', error);
            } finally {
                setIsLoadingActivity(false);
            }
        }
        fetchActivity();
    }, [user?.token]);

    // Fetch Phase 2, 3 & 4 Data
    useEffect(() => {
        if (!user?.token) return;
        async function loadPhase2() {
            try {
                const [revData, tktData, affData, cmpData, flgData, emgData] = await Promise.all([
                    api.finance.getRevenue().catch(() => null),
                    api.support.getTickets().catch(() => []),
                    api.affiliates.getList().catch(() => []),
                    api.operations.getCampaigns().catch(() => []),
                    api.system.getFlags().catch(() => []),
                    api.emergency.getRequests().catch(() => [])
                ]);
                setRevenue(revData);
                setTickets(tktData);
                setAffiliates(affData);
                setCampaigns(cmpData);
                setFlags(flgData);
                setEmergencyRequests(emgData);
            } catch (e) {
                console.error("Failed to load Phase 2/3/4 data", e);
            }
        }
        loadPhase2();
    }, [user?.token]);

    // Fetch Policies
    useEffect(() => {
        if (!user?.token) return;
        async function fetchPolicies() {
            try {
                const data = await api.admin.getPolicies();
                setPolicies(data.policies || []);
            } catch (error) {
                console.error('Failed to fetch policies:', error);
            } finally {
                setIsLoadingPolicies(false);
            }
        }
        fetchPolicies();
        fetchPolicies();
    }, [user?.token]);

    const handleCreateUser = async () => {
        setIsProcessing('create-user');
        try {
            await api.admin.createUser(newUser);
            toast({ title: 'User created', description: `${newUser.username} added successfully` });
            setUsers([...users, { username: newUser.username, role: newUser.role as 'admin' | 'user' }]);
            setShowCreateUser(false);
            setNewUser({ username: '', password: '', role: 'user' });
        } catch (error) {
            toast({ variant: 'destructive', title: 'Failed to create user', description: String(error) });
        } finally {
            setIsProcessing(null);
        }
    };

    const handleCreateAffiliate = async () => {
        setIsProcessing('create-affiliate');
        try {
            await api.affiliates.create(newAffiliate);
            toast({ title: 'Affiliate created', description: `${newAffiliate.code} added successfully` });
            setAffiliates([...affiliates, { ...newAffiliate, total_earnings: 0, active: true }]);
            setShowCreateAffiliate(false);
            setNewAffiliate({ code: '', user: '', commission_rate: 0.1 });
        } catch (error) {
            toast({ variant: 'destructive', title: 'Failed to create affiliate', description: String(error) });
        } finally {
            setIsProcessing(null);
        }
    };

    const handleDeleteAffiliate = async (code: string) => {
         if (!confirm('Are you sure you want to delete this affiliate?')) return;
         setIsProcessing(code);
         try {
             await api.affiliates.delete(code);
             setAffiliates(affiliates.filter(a => a.code !== code));
             toast({ title: 'Affiliate deleted' });
         } catch (error) {
             toast({ variant: 'destructive', title: 'Failed to delete affiliate', description: String(error) });
         } finally {
             setIsProcessing(null);
         }
    };

    const handleCreateCampaign = async () => {
        setIsProcessing('create-campaign');
        try {
            await api.operations.createCampaign(newCampaign);
            toast({ title: 'Campaign created', description: `${newCampaign.name} added successfully` });
            setCampaigns([...campaigns, { ...newCampaign, conversion_rate: 0, created_at: new Date().toISOString() } as any]);
            setShowCreateCampaign(false);
            setNewCampaign({ id: '', name: '', type: 'email', target_audience: 'all', status: 'draft' });
        } catch (error) {
             toast({ variant: 'destructive', title: 'Failed to create campaign', description: String(error) });
        } finally {
            setIsProcessing(null);
        }
    };

    const handleDeleteCampaign = async (id: string) => {
         if (!confirm('Are you sure you want to delete this campaign?')) return;
         setIsProcessing(id);
         try {
             await api.operations.deleteCampaign(id);
             setCampaigns(campaigns.filter(c => c.id !== id));
             toast({ title: 'Campaign deleted' });
         } catch (error) {
             toast({ variant: 'destructive', title: 'Failed to delete campaign', description: String(error) });
         } finally {
             setIsProcessing(null);
         }
    };

    const handleToggleRole = async (username: string, currentRole: string) => {
        const newRole = currentRole === 'admin' ? 'user' : 'admin';
        setIsProcessing(username);

        try {
            await api.admin.updateUserRole(username, newRole);
            setUsers(users.map(u =>
                u.username === username ? { ...u, role: newRole as 'admin' | 'user' } : u
            ));
            toast({
                title: 'Role updated',
                description: `${username} is now a ${newRole}`,
            });
        } catch (error) {
            toast({
                variant: 'destructive',
                title: 'Failed to update role',
                description: String(error),
            });
        } finally {
            setIsProcessing(null);
        }
    };

    const handleDeleteUser = async (username: string) => {
        if (!confirm(`Are you sure you want to delete user "${username}"?`)) return;

        setIsProcessing(username);

        try {
            await api.admin.deleteUser(username);
            setUsers(users.filter(u => u.username !== username));
            toast({
                title: 'User deleted',
                description: `${username} has been removed`,
            });
        } catch (error) {
            toast({
                variant: 'destructive',
                title: 'Failed to delete user',
                description: String(error),
            });
        } finally {
            setIsProcessing(null);
        }
    };

    const handleTriggerReflection = async () => {
        setIsProcessing('reflection');

        try {
            await api.admin.triggerReflection();
            toast({
                title: 'Reflection triggered',
                description: 'The memory kernel is processing a reflection cycle',
            });
        } catch (error) {
            toast({
                variant: 'destructive',
                title: 'Failed to trigger reflection',
            });
        } finally {
            setIsProcessing(null);
        }
    };

    const viewUserDetails = async (username: string) => {
        setIsProcessing(username);

        try {
            const data = await api.admin.getUserDetails(username);
            setSelectedUser(data);
            setShowUserDetails(true);
        } catch (error) {
            toast({
                variant: 'destructive',
                title: 'Failed to load user details',
            });
        } finally {
            setIsProcessing(null);
        }
    };

    const handleExtendTrial = async (username: string, days: number) => {
        setIsProcessing('trial-' + username);

        try {
            const data = await api.admin.extendTrial(username, days);
            toast({
                title: 'Trial extended',
                description: `${username}'s trial extended until ${new Date(data.expires_at).toLocaleDateString()}`,
            });
            // Refresh user details
            if (selectedUser?.username === username) {
                viewUserDetails(username);
            }
        } catch (error) {
            toast({
                variant: 'destructive',
                title: 'Failed to extend trial',
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
        if (selectedUsernames.size === 0) return;
        setIsProcessing('batch-role');

        try {
            const data = await api.admin.batchUpdateRole(Array.from(selectedUsernames), role);
            toast({
                title: 'Batch update complete',
                description: `Updated ${data.updated} users to ${role}`,
            });
            // Refresh users
            setSelectedUsernames(new Set());
            window.location.reload();
        } catch (error) {
            toast({ variant: 'destructive', title: 'Batch update failed' });
        } finally {
            setIsProcessing(null);
        }
    };

    // Batch delete
    const handleBatchDelete = async () => {
        if (selectedUsernames.size === 0) return;
        if (!confirm(`Delete ${selectedUsernames.size} users? This cannot be undone.`)) return;

        setIsProcessing('batch-delete');

        try {
            const data = await api.admin.batchDeleteUsers(Array.from(selectedUsernames));
            toast({
                title: 'Batch delete complete',
                description: `Deleted ${data.deleted} users`,
            });
            setSelectedUsernames(new Set());
            window.location.reload();
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
                            <TabsTrigger value="finance" className="gap-2">
                                <DollarSign className="w-4 h-4" />
                                Finance
                            </TabsTrigger>
                            <TabsTrigger value="support" className="gap-2">
                                <LifeBuoy className="w-4 h-4" />
                                Support
                            </TabsTrigger>
                            <TabsTrigger value="affiliates" className="gap-2">
                                <Share2 className="w-4 h-4" />
                                Affiliates
                            </TabsTrigger>
                            <TabsTrigger value="operations" className="gap-2">
                                <Megaphone className="w-4 h-4" />
                                Operations
                            </TabsTrigger>
                            <TabsTrigger value="policies" className="gap-2">
                                <Lock className="w-4 h-4" />
                                Policies
                            </TabsTrigger>
                            <TabsTrigger value="emergency" className="gap-2 text-red-400 data-[state=active]:text-red-300">
                                <AlertTriangle className="w-4 h-4" />
                                Emergency
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
                                        <Button size="sm" onClick={() => setShowCreateUser(true)}>
                                            <Plus className="w-4 h-4 mr-2" />
                                            Add User
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

                                    {/* Create User Modal */}
                                    {showCreateUser && (
                                        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowCreateUser(false)}>
                                            <div className="bg-background rounded-xl p-6 max-w-md w-full mx-4 border border-border shadow-2xl" onClick={(e) => e.stopPropagation()}>
                                                <div className="flex items-center justify-between mb-4">
                                                    <h3 className="text-lg font-semibold">Add New User</h3>
                                                    <button onClick={() => setShowCreateUser(false)} className="text-muted-foreground hover:text-foreground">
                                                        <X className="w-5 h-5" />
                                                    </button>
                                                </div>
                                                <div className="space-y-4">
                                                    <div>
                                                        <label className="text-sm font-medium mb-1 block">Username</label>
                                                        <input
                                                            type="text"
                                                            value={newUser.username}
                                                            onChange={(e) => setNewUser({...newUser, username: e.target.value})}
                                                            className="w-full px-3 py-2 rounded-lg border border-border bg-muted/30"
                                                            placeholder="jdoe"
                                                        />
                                                    </div>
                                                    <div>
                                                        <label className="text-sm font-medium mb-1 block">Password</label>
                                                        <input
                                                            type="password"
                                                            value={newUser.password}
                                                            onChange={(e) => setNewUser({...newUser, password: e.target.value})}
                                                            className="w-full px-3 py-2 rounded-lg border border-border bg-muted/30"
                                                            placeholder="••••••••"
                                                        />
                                                    </div>
                                                    <div>
                                                        <label className="text-sm font-medium mb-1 block">Role</label>
                                                        <select
                                                            value={newUser.role}
                                                            onChange={(e) => setNewUser({...newUser, role: e.target.value})}
                                                            className="w-full px-3 py-2 rounded-lg border border-border bg-muted/30"
                                                        >
                                                            <option value="user">User</option>
                                                            <option value="admin">Admin</option>
                                                        </select>
                                                    </div>
                                                    <div className="pt-2 flex justify-end gap-2">
                                                        <Button variant="ghost" onClick={() => setShowCreateUser(false)}>Cancel</Button>
                                                        <Button onClick={handleCreateUser} disabled={!newUser.username || !newUser.password || isProcessing === 'create-user'}>
                                                            {isProcessing === 'create-user' ? <Loader2 className="w-4 h-4 animate-spin" /> : 'Create User'}
                                                        </Button>
                                                    </div>
                                                </div>
                                            </div>
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
                                                            <p className="text-2xl font-bold">{String(stats?.kernel_stats?.total_groups || 0)}</p>
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
                                                            <p className="text-2xl font-bold">{String(stats?.kernel_stats?.active_groups || 0)}</p>
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

                        {/* Finance Tab */}
                        <TabsContent value="finance" className="space-y-6">
                            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                                <Card className="border-cyan-500/20 bg-cyan-900/10 backdrop-blur-sm">
                                    <div className="p-6">
                                        <h3 className="text-lg font-semibold mb-2">Total Revenue</h3>
                                        <p className="text-3xl font-bold text-cyan-400">
                                            ${revenue?.total_revenue?.toLocaleString() || '0.00'}
                                        </p>
                                    </div>
                                </Card>
                                <Card className="border-indigo-500/20 bg-indigo-900/10 backdrop-blur-sm">
                                    <div className="p-6">
                                        <h3 className="text-lg font-semibold mb-2">MRR</h3>
                                        <p className="text-3xl font-bold text-indigo-400">
                                            ${revenue?.mrr?.toLocaleString() || '0.00'}
                                        </p>
                                    </div>
                                </Card>
                                <Card className="border-amber-500/20 bg-amber-900/10 backdrop-blur-sm">
                                    <div className="p-6">
                                        <h3 className="text-lg font-semibold mb-2">ARR</h3>
                                        <p className="text-3xl font-bold text-amber-400">
                                            ${revenue?.arr?.toLocaleString() || '0.00'}
                                        </p>
                                    </div>
                                </Card>
                            </div>
                        </TabsContent>

                        {/* Support Tab */}
                        <TabsContent value="support" className="space-y-6">
                            <Card className="border-indigo-500/20 bg-black/40 backdrop-blur-xl">
                                <div className="p-6">
                                    <h2 className="text-xl font-bold mb-4">Support Tickets</h2>
                                    <div className="space-y-4">
                                        {tickets.map(t => (
                                            <div key={t.id} className="flex justify-between items-center p-4 rounded-lg bg-white/5 border border-white/10">
                                                <div>
                                                    <h4 className="font-semibold text-indigo-300">{t.title}</h4>
                                                    <div className="text-sm text-gray-400 flex gap-4 mt-1">
                                                        <span>Priority: {t.priority}</span>
                                                        <span>Status: {t.status}</span>
                                                        <span>From: {t.created_by}</span>
                                                    </div>
                                                </div>
                                                <Button variant="outline" size="sm" onClick={() => api.support.resolveTicket(t.id)}>
                                                    Resolve
                                                </Button>
                                            </div>
                                        ))}
                                        {tickets.length === 0 && <p className="text-gray-500">No active tickets</p>}
                                    </div>
                                </div>
                            </Card>
                        </TabsContent>

                        {/* Affiliates Tab */}
                        <TabsContent value="affiliates" className="space-y-6">
                            <Card className="border-indigo-500/20 bg-black/40 backdrop-blur-xl">
                                <div className="p-6">
                                    <div className="flex justify-between items-center mb-4">
                                        <h2 className="text-xl font-bold">Affiliate Program</h2>
                                        <Button size="sm" onClick={() => setShowCreateAffiliate(true)}>
                                            <Plus className="w-4 h-4 mr-2" />
                                            Add Affiliate
                                        </Button>
                                    </div>
                                    <div className="space-y-4">
                                        {affiliates.map(a => (
                                            <div key={a.code} className="flex justify-between items-center p-4 rounded-lg bg-white/5 border border-white/10">
                                                <div>
                                                    <h4 className="font-semibold text-emerald-300">{a.code}</h4>
                                                    <div className="text-sm text-gray-400 flex gap-4 mt-1">
                                                        <span>User: {a.user}</span>
                                                        <span>Rate: {a.commission_rate * 100}%</span>
                                                        <span>Earnings: ${a.total_earnings}</span>
                                                    </div>
                                                </div>
                                                <div className="flex items-center gap-2">
                                                    <Badge variant={a.active ? 'default' : 'secondary'}>
                                                        {a.active ? 'Active' : 'Inactive'}
                                                    </Badge>
                                                    <Button variant="ghost" size="icon" onClick={() => handleDeleteAffiliate(a.code)} disabled={isProcessing === a.code}>
                                                        {isProcessing === a.code ? <Loader2 className="w-4 h-4 animate-spin" /> : <Trash2 className="w-4 h-4 text-red-400" />}
                                                    </Button>
                                                </div>
                                            </div>
                                        ))}
                                        {affiliates.length === 0 && <p className="text-gray-500">No affiliates found</p>}
                                    </div>
                                </div>
                            </Card>

                             {/* Create Affiliate Modal */}
                             {showCreateAffiliate && (
                                <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowCreateAffiliate(false)}>
                                    <div className="bg-background rounded-xl p-6 max-w-md w-full mx-4 border border-border shadow-2xl" onClick={(e) => e.stopPropagation()}>
                                        <div className="flex items-center justify-between mb-4">
                                            <h3 className="text-lg font-semibold">Add New Affiliate</h3>
                                            <button onClick={() => setShowCreateAffiliate(false)} className="text-muted-foreground hover:text-foreground">
                                                <X className="w-5 h-5" />
                                            </button>
                                        </div>
                                        <div className="space-y-4">
                                            <div>
                                                <label className="text-sm font-medium mb-1 block">Affiliate Code</label>
                                                <input
                                                    type="text"
                                                    value={newAffiliate.code}
                                                    onChange={(e) => setNewAffiliate({...newAffiliate, code: e.target.value})}
                                                    className="w-full px-3 py-2 rounded-lg border border-border bg-muted/30"
                                                    placeholder="SUMMER2025"
                                                />
                                            </div>
                                            <div>
                                                <label className="text-sm font-medium mb-1 block">User</label>
                                                <input
                                                    type="text"
                                                    value={newAffiliate.user}
                                                    onChange={(e) => setNewAffiliate({...newAffiliate, user: e.target.value})}
                                                    className="w-full px-3 py-2 rounded-lg border border-border bg-muted/30"
                                                    placeholder="username"
                                                />
                                            </div>
                                            <div>
                                                <label className="text-sm font-medium mb-1 block">Commission Rate</label>
                                                <input
                                                    type="number"
                                                    step="0.01"
                                                    value={newAffiliate.commission_rate}
                                                    onChange={(e) => setNewAffiliate({...newAffiliate, commission_rate: Number(e.target.value)})}
                                                    className="w-full px-3 py-2 rounded-lg border border-border bg-muted/30"
                                                />
                                            </div>
                                            <div className="pt-2 flex justify-end gap-2">
                                                <Button variant="ghost" onClick={() => setShowCreateAffiliate(false)}>Cancel</Button>
                                                <Button onClick={handleCreateAffiliate} disabled={!newAffiliate.code || !newAffiliate.user || isProcessing === 'create-affiliate'}>
                                                    {isProcessing === 'create-affiliate' ? <Loader2 className="w-4 h-4 animate-spin" /> : 'Create Affiliate'}
                                                </Button>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            )}
                        </TabsContent>

                        {/* Operations Tab */}
                        <TabsContent value="operations" className="space-y-6">
                             <Card className="border-indigo-500/20 bg-black/40 backdrop-blur-xl">
                                <div className="p-6">
                                    <div className="flex justify-between items-center mb-4">
                                        <h2 className="text-xl font-bold">Marketing Campaigns</h2>
                                        <Button size="sm" onClick={() => setShowCreateCampaign(true)}>
                                            <Plus className="w-4 h-4 mr-2" />
                                            Create Campaign
                                        </Button>
                                    </div>
                                    <div className="space-y-4">
                                        {campaigns.map(c => (
                                            <div key={c.id} className="flex justify-between items-center p-4 rounded-lg bg-white/5 border border-white/10">
                                                <div>
                                                    <h4 className="font-semibold text-pink-300">{c.name}</h4>
                                                    <div className="text-sm text-gray-400 flex gap-4 mt-1">
                                                        <span>Type: {c.type}</span>
                                                        <span>Audience: {c.target_audience}</span>
                                                        <span>Conv: {(c.conversion_rate * 100).toFixed(1)}%</span>
                                                    </div>
                                                </div>
                                                <div className="flex items-center gap-2">
                                                    <Badge>{c.status}</Badge>
                                                    <Button variant="ghost" size="icon" onClick={() => handleDeleteCampaign(c.id)} disabled={isProcessing === c.id}>
                                                        {isProcessing === c.id ? <Loader2 className="w-4 h-4 animate-spin" /> : <Trash2 className="w-4 h-4 text-red-400" />}
                                                    </Button>
                                                </div>
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            </Card>

                            {/* Create Campaign Modal */}
                            {showCreateCampaign && (
                                <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowCreateCampaign(false)}>
                                    <div className="bg-background rounded-xl p-6 max-w-md w-full mx-4 border border-border shadow-2xl" onClick={(e) => e.stopPropagation()}>
                                        <div className="flex items-center justify-between mb-4">
                                            <h3 className="text-lg font-semibold">Create New Campaign</h3>
                                            <button onClick={() => setShowCreateCampaign(false)} className="text-muted-foreground hover:text-foreground">
                                                <X className="w-5 h-5" />
                                            </button>
                                        </div>
                                        <div className="space-y-4">
                                            <div>
                                                <label className="text-sm font-medium mb-1 block">Campaign ID</label>
                                                <input
                                                    type="text"
                                                    value={newCampaign.id}
                                                    onChange={(e) => setNewCampaign({...newCampaign, id: e.target.value})}
                                                    className="w-full px-3 py-2 rounded-lg border border-border bg-muted/30"
                                                    placeholder="cmp_new"
                                                />
                                            </div>
                                            <div>
                                                <label className="text-sm font-medium mb-1 block">Name</label>
                                                <input
                                                    type="text"
                                                    value={newCampaign.name}
                                                    onChange={(e) => setNewCampaign({...newCampaign, name: e.target.value})}
                                                    className="w-full px-3 py-2 rounded-lg border border-border bg-muted/30"
                                                    placeholder="Campaign Name"
                                                />
                                            </div>
                                            <div>
                                                <label className="text-sm font-medium mb-1 block">Type</label>
                                                <select
                                                    value={newCampaign.type}
                                                    onChange={(e) => setNewCampaign({...newCampaign, type: e.target.value})}
                                                    className="w-full px-3 py-2 rounded-lg border border-border bg-muted/30"
                                                >
                                                    <option value="email">Email</option>
                                                    <option value="in-app">In-App</option>
                                                    <option value="push">Push Notification</option>
                                                </select>
                                            </div>
                                            <div>
                                                <label className="text-sm font-medium mb-1 block">Status</label>
                                                 <select
                                                    value={newCampaign.status}
                                                    onChange={(e) => setNewCampaign({...newCampaign, status: e.target.value})}
                                                    className="w-full px-3 py-2 rounded-lg border border-border bg-muted/30"
                                                >
                                                    <option value="draft">Draft</option>
                                                    <option value="active">Active</option>
                                                </select>
                                            </div>
                                            <div className="pt-2 flex justify-end gap-2">
                                                <Button variant="ghost" onClick={() => setShowCreateCampaign(false)}>Cancel</Button>
                                                <Button onClick={handleCreateCampaign} disabled={!newCampaign.id || !newCampaign.name || isProcessing === 'create-campaign'}>
                                                    {isProcessing === 'create-campaign' ? <Loader2 className="w-4 h-4 animate-spin" /> : 'Create Campaign'}
                                                </Button>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            )}
                        </TabsContent>

                        {/* System Tab */}
                        <TabsContent value="system" className="space-y-6">
                             <Card className="border-indigo-500/20 bg-black/40 backdrop-blur-xl">
                                <div className="p-6">
                                    <h2 className="text-xl font-bold mb-4">Feature Flags</h2>
                                    <div className="space-y-4">
                                        {flags.map(f => (
                                            <div key={f.key} className="flex justify-between items-center p-4 rounded-lg bg-white/5 border border-white/10">
                                                <div>
                                                    <h4 className="font-semibold text-blue-300">{f.name}</h4>
                                                    <p className="text-sm text-gray-400">{f.description}</p>
                                                </div>
                                                <div className="flex items-center gap-2">
                                                    <Badge variant={f.is_enabled ? 'default' : 'secondary'}>
                                                        {f.is_enabled ? 'Active' : 'Disabled'}
                                                    </Badge>
                                                    <Button variant="ghost" size="sm" onClick={async () => {
                                                        await api.system.toggleFlag(f.key, !f.is_enabled);
                                                        setFlags(flags.map(fl => fl.key === f.key ? { ...fl, is_enabled: !f.is_enabled } : fl));
                                                    }}>Toggle</Button>
                                                </div>
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            </Card>
                        </TabsContent>

                        {/* Emergency Tab */}
                        <TabsContent value="emergency" className="space-y-6">
                             <Card className="border-red-500/20 bg-red-900/10 backdrop-blur-xl">
                                <div className="p-6">
                                    <div className="flex items-center gap-3 mb-4">
                                        <AlertTriangle className="w-6 h-6 text-red-500" />
                                        <h2 className="text-xl font-bold text-red-100">Emergency Access Requests</h2>
                                    </div>
                                    <p className="text-gray-400 mb-6">Manage strictly audited privileged access requests here.</p>
                                    
                                    <div className="space-y-4">
                                        {emergencyRequests.map(r => (
                                            <div key={r.id} className="flex justify-between items-center p-4 rounded-lg bg-red-500/5 border border-red-500/10 hover:border-red-500/30 transition-colors">
                                                <div>
                                                    <h4 className="font-semibold text-red-200">{r.reason}</h4>
                                                    <div className="text-sm text-gray-400 flex gap-4 mt-1">
                                                        <span>User: {r.requested_by}</span>
                                                        <span>Duration: {r.duration}</span>
                                                    </div>
                                                </div>
                                                <div className="flex items-center gap-2">
                                                    <Badge variant={r.status === 'pending' ? 'outline' : 'secondary'} className={r.status === 'pending' ? 'text-yellow-400 border-yellow-400/50' : ''}>
                                                        {r.status.toUpperCase()}
                                                    </Badge>
                                                    {r.status === 'pending' && (
                                                        <div className="flex gap-2 ml-4">
                                                            <Button size="sm" variant="destructive" onClick={async () => {
                                                                await api.emergency.deny(r.id);
                                                                setEmergencyRequests(emergencyRequests.map(er => er.id === r.id ? { ...er, status: 'denied' } : er));
                                                            }}>Deny</Button>
                                                            <Button size="sm" className="bg-red-600 hover:bg-red-700" onClick={async () => {
                                                                await api.emergency.approve(r.id);
                                                                setEmergencyRequests(emergencyRequests.map(er => er.id === r.id ? { ...er, status: 'approved' } : er));
                                                            }}>Approve</Button>
                                                        </div>
                                                    )}
                                                </div>
                                            </div>
                                        ))}
                                        {emergencyRequests.length === 0 && <p className="text-gray-500">No pending emergency requests</p>}
                                    </div>
                                </div>
                            </Card>
                        </TabsContent>

                        {/* Policies Tab */}
                        <TabsContent value="policies">
                            <Card>
                                <CardHeader>
                                    <div className="flex justify-between items-center">
                                        <div>
                                            <CardTitle className="flex items-center gap-2">
                                                <Lock className="w-5 h-5" />
                                                Access Control Policies
                                            </CardTitle>
                                            <CardDescription>
                                                Manage ALLOW/DENY policies for fine-grained access control
                                            </CardDescription>
                                        </div>
                                        <Button onClick={() => setShowCreatePolicy(true)} className="gap-2">
                                            <Plus className="w-4 h-4" />
                                            Create Policy
                                        </Button>
                                    </div>
                                </CardHeader>
                                <CardContent>
                                    {isLoadingPolicies ? (
                                        <div className="flex justify-center py-8">
                                            <Loader2 className="w-6 h-6 animate-spin text-primary" />
                                        </div>
                                    ) : policies.length === 0 ? (
                                        <div className="text-center py-12">
                                            <Lock className="w-12 h-12 mx-auto mb-4 text-muted-foreground/50" />
                                            <p className="text-muted-foreground">No policies defined</p>
                                            <p className="text-sm text-muted-foreground/70 mt-1">Create your first policy to control access to resources</p>
                                        </div>
                                    ) : (
                                        <div className="space-y-4">
                                            {policies.map((p) => (
                                                <div
                                                    key={p.id}
                                                    className={`p-4 rounded-lg border ${
                                                        p.effect === 'DENY' 
                                                            ? 'border-red-500/30 bg-red-500/5' 
                                                            : 'border-green-500/30 bg-green-500/5'
                                                    }`}
                                                >
                                                    <div className="flex justify-between items-start">
                                                        <div className="flex-1">
                                                            <div className="flex items-center gap-3 mb-2">
                                                                <Badge 
                                                                    variant={p.effect === 'DENY' ? 'destructive' : 'default'}
                                                                    className={p.effect === 'ALLOW' ? 'bg-green-600' : ''}
                                                                >
                                                                    {p.effect}
                                                                </Badge>
                                                                <span className="font-mono text-sm text-muted-foreground">{p.id}</span>
                                                            </div>
                                                            <p className="text-sm mb-3">{p.description || 'No description'}</p>
                                                            <div className="grid grid-cols-3 gap-4 text-sm">
                                                                <div>
                                                                    <p className="text-muted-foreground text-xs mb-1">Subjects</p>
                                                                    <div className="flex flex-wrap gap-1">
                                                                        {p.subjects.map((s, i) => (
                                                                            <span key={i} className="px-2 py-0.5 rounded bg-muted text-xs">{s}</span>
                                                                        ))}
                                                                    </div>
                                                                </div>
                                                                <div>
                                                                    <p className="text-muted-foreground text-xs mb-1">Resources</p>
                                                                    <div className="flex flex-wrap gap-1">
                                                                        {p.resources.map((r, i) => (
                                                                            <span key={i} className="px-2 py-0.5 rounded bg-muted text-xs">{r}</span>
                                                                        ))}
                                                                    </div>
                                                                </div>
                                                                <div>
                                                                    <p className="text-muted-foreground text-xs mb-1">Actions</p>
                                                                    <div className="flex flex-wrap gap-1">
                                                                        {p.actions.map((a, i) => (
                                                                            <span key={i} className="px-2 py-0.5 rounded bg-muted text-xs">{a}</span>
                                                                        ))}
                                                                    </div>
                                                                </div>
                                                            </div>
                                                        </div>
                                                        <Button
                                                            variant="ghost"
                                                            size="sm"
                                                            className="text-red-500 hover:text-red-400 hover:bg-red-500/10"
                                                            onClick={async () => {
                                                                if (!confirm(`Delete policy "${p.id}"?`)) return;
                                                                try {
                                                                    await api.admin.deletePolicy(p.id);
                                                                    setPolicies(policies.filter(pol => pol.id !== p.id));
                                                                    toast({ title: 'Policy deleted', description: `Policy ${p.id} has been removed` });
                                                                } catch (error) {
                                                                    toast({ variant: 'destructive', title: 'Failed to delete policy' });
                                                                }
                                                            }}
                                                        >
                                                            <Trash2 className="w-4 h-4" />
                                                        </Button>
                                                    </div>
                                                </div>
                                            ))}
                                        </div>
                                    )}

                                    {/* Create Policy Modal */}
                                    {showCreatePolicy && (
                                        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowCreatePolicy(false)}>
                                            <div className="bg-background rounded-xl p-6 max-w-lg w-full mx-4 border border-border shadow-2xl max-h-[80vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
                                                <div className="flex items-center justify-between mb-4">
                                                    <h3 className="text-lg font-semibold">Create New Policy</h3>
                                                    <button onClick={() => setShowCreatePolicy(false)} className="text-muted-foreground hover:text-foreground">
                                                        <X className="w-5 h-5" />
                                                    </button>
                                                </div>
                                                <div className="space-y-4">
                                                    <div>
                                                        <label className="text-sm font-medium mb-1 block">Policy ID</label>
                                                        <input
                                                            type="text"
                                                            value={newPolicy.id}
                                                            onChange={(e) => setNewPolicy({...newPolicy, id: e.target.value})}
                                                            placeholder="e.g., deny_confidential_data"
                                                            className="w-full px-3 py-2 rounded-lg border border-border bg-muted/30 focus:outline-none focus:ring-2 focus:ring-primary/50"
                                                        />
                                                    </div>
                                                    <div>
                                                        <label className="text-sm font-medium mb-1 block">Description</label>
                                                        <input
                                                            type="text"
                                                            value={newPolicy.description}
                                                            onChange={(e) => setNewPolicy({...newPolicy, description: e.target.value})}
                                                            placeholder="What does this policy do?"
                                                            className="w-full px-3 py-2 rounded-lg border border-border bg-muted/30 focus:outline-none focus:ring-2 focus:ring-primary/50"
                                                        />
                                                    </div>
                                                    <div>
                                                        <label className="text-sm font-medium mb-1 block">Effect</label>
                                                        <select
                                                            value={newPolicy.effect}
                                                            onChange={(e) => setNewPolicy({...newPolicy, effect: e.target.value as 'ALLOW' | 'DENY'})}
                                                            className="w-full px-3 py-2 rounded-lg border border-border bg-muted/30 focus:outline-none focus:ring-2 focus:ring-primary/50"
                                                        >
                                                            <option value="DENY">DENY</option>
                                                            <option value="ALLOW">ALLOW</option>
                                                        </select>
                                                    </div>
                                                    <div>
                                                        <label className="text-sm font-medium mb-1 block">Subjects (user:id or group:name)</label>
                                                        <div className="flex gap-2 mb-2">
                                                            <input
                                                                type="text"
                                                                value={subjectInput}
                                                                onChange={(e) => setSubjectInput(e.target.value)}
                                                                placeholder="e.g., user:john or group:admins"
                                                                className="flex-1 px-3 py-2 rounded-lg border border-border bg-muted/30 focus:outline-none focus:ring-2 focus:ring-primary/50"
                                                                onKeyDown={(e) => {
                                                                    if (e.key === 'Enter' && subjectInput.trim()) {
                                                                        setNewPolicy({...newPolicy, subjects: [...newPolicy.subjects, subjectInput.trim()]});
                                                                        setSubjectInput('');
                                                                    }
                                                                }}
                                                            />
                                                            <Button variant="outline" onClick={() => {
                                                                if (subjectInput.trim()) {
                                                                    setNewPolicy({...newPolicy, subjects: [...newPolicy.subjects, subjectInput.trim()]});
                                                                    setSubjectInput('');
                                                                }
                                                            }}>Add</Button>
                                                        </div>
                                                        <div className="flex flex-wrap gap-1">
                                                            {newPolicy.subjects.map((s, i) => (
                                                                <span key={i} className="px-2 py-1 rounded bg-muted text-xs flex items-center gap-1">
                                                                    {s}
                                                                    <button onClick={() => setNewPolicy({...newPolicy, subjects: newPolicy.subjects.filter((_, idx) => idx !== i)})} className="text-muted-foreground hover:text-red-400">×</button>
                                                                </span>
                                                            ))}
                                                        </div>
                                                    </div>
                                                    <div>
                                                        <label className="text-sm font-medium mb-1 block">Resources (node:id, type:NodeType, or *)</label>
                                                        <div className="flex gap-2 mb-2">
                                                            <input
                                                                type="text"
                                                                value={resourceInput}
                                                                onChange={(e) => setResourceInput(e.target.value)}
                                                                placeholder="e.g., type:Fact or *"
                                                                className="flex-1 px-3 py-2 rounded-lg border border-border bg-muted/30 focus:outline-none focus:ring-2 focus:ring-primary/50"
                                                                onKeyDown={(e) => {
                                                                    if (e.key === 'Enter' && resourceInput.trim()) {
                                                                        setNewPolicy({...newPolicy, resources: [...newPolicy.resources, resourceInput.trim()]});
                                                                        setResourceInput('');
                                                                    }
                                                                }}
                                                            />
                                                            <Button variant="outline" onClick={() => {
                                                                if (resourceInput.trim()) {
                                                                    setNewPolicy({...newPolicy, resources: [...newPolicy.resources, resourceInput.trim()]});
                                                                    setResourceInput('');
                                                                }
                                                            }}>Add</Button>
                                                        </div>
                                                        <div className="flex flex-wrap gap-1">
                                                            {newPolicy.resources.map((r, i) => (
                                                                <span key={i} className="px-2 py-1 rounded bg-muted text-xs flex items-center gap-1">
                                                                    {r}
                                                                    <button onClick={() => setNewPolicy({...newPolicy, resources: newPolicy.resources.filter((_, idx) => idx !== i)})} className="text-muted-foreground hover:text-red-400">×</button>
                                                                </span>
                                                            ))}
                                                        </div>
                                                    </div>
                                                    <div>
                                                        <label className="text-sm font-medium mb-1 block">Actions</label>
                                                        <div className="flex flex-wrap gap-2">
                                                            {['READ', 'WRITE', 'DELETE', 'ADMIN'].map((action) => (
                                                                <label key={action} className="flex items-center gap-2">
                                                                    <input
                                                                        type="checkbox"
                                                                        checked={newPolicy.actions.includes(action)}
                                                                        onChange={(e) => {
                                                                            if (e.target.checked) {
                                                                                setNewPolicy({...newPolicy, actions: [...newPolicy.actions, action]});
                                                                            } else {
                                                                                setNewPolicy({...newPolicy, actions: newPolicy.actions.filter(a => a !== action)});
                                                                            }
                                                                        }}
                                                                        className="w-4 h-4 rounded"
                                                                    />
                                                                    <span className="text-sm">{action}</span>
                                                                </label>
                                                            ))}
                                                        </div>
                                                    </div>
                                                    <div className="pt-4 border-t border-border flex justify-end gap-2">
                                                        <Button variant="outline" onClick={() => setShowCreatePolicy(false)}>Cancel</Button>
                                                        <Button
                                                            disabled={!newPolicy.id || newPolicy.subjects.length === 0 || newPolicy.resources.length === 0 || newPolicy.actions.length === 0}
                                                            onClick={async () => {
                                                                try {
                                                                    await api.admin.createPolicy(newPolicy);
                                                                    setPolicies([...policies, newPolicy]);
                                                                    setShowCreatePolicy(false);
                                                                    setNewPolicy({ id: '', description: '', subjects: [], resources: [], actions: [], effect: 'DENY' });
                                                                    toast({ title: 'Policy created', description: `Policy ${newPolicy.id} has been created` });
                                                                } catch (error) {
                                                                    toast({ variant: 'destructive', title: 'Failed to create policy' });
                                                                }
                                                            }}
                                                        >
                                                            Create Policy
                                                        </Button>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    )}
                                </CardContent>
                            </Card>
                        </TabsContent>

                    </Tabs>
                </motion.div>
            </main>
        </div>
    );
}
