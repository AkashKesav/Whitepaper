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
} from 'lucide-react';

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:3000';

interface AdminUser {
    username: string;
    role: 'admin' | 'user';
}

interface SystemStats {
    total_users: number;
    total_admins: number;
    timestamp: string;
    kernel_stats?: Record<string, unknown>;
}

interface ActivityEntry {
    timestamp: string;
    user_id: string;
    action: string;
    details: string;
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
                        <TabsList className="grid w-full grid-cols-4 max-w-lg">
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
                                    {isLoadingUsers ? (
                                        <div className="flex justify-center py-8">
                                            <Loader2 className="w-6 h-6 animate-spin text-primary" />
                                        </div>
                                    ) : users.length === 0 ? (
                                        <p className="text-muted-foreground text-center py-8">No users found</p>
                                    ) : (
                                        <div className="space-y-3">
                                            {users.map((u) => (
                                                <div
                                                    key={u.username}
                                                    className="flex items-center justify-between p-4 rounded-lg border border-border/50 bg-muted/30"
                                                >
                                                    <div className="flex items-center gap-3">
                                                        <div className="w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center">
                                                            {u.role === 'admin' ? (
                                                                <ShieldCheck className="w-5 h-5 text-primary" />
                                                            ) : (
                                                                <Users className="w-5 h-5 text-muted-foreground" />
                                                            )}
                                                        </div>
                                                        <div>
                                                            <p className="font-medium">{u.username}</p>
                                                            <Badge variant={u.role === 'admin' ? 'default' : 'secondary'} className="text-xs">
                                                                {u.role}
                                                            </Badge>
                                                        </div>
                                                    </div>
                                                    <div className="flex items-center gap-2">
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
                            </div>
                        </TabsContent>

                        {/* Groups Tab */}
                        <TabsContent value="groups">
                            <Card>
                                <CardHeader>
                                    <CardTitle>Groups Overview</CardTitle>
                                    <CardDescription>All groups in the system</CardDescription>
                                </CardHeader>
                                <CardContent>
                                    <p className="text-muted-foreground text-center py-8">
                                        Groups management coming soon. Use the regular groups interface for now.
                                    </p>
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
                                                    className="flex items-start gap-3 p-3 rounded-lg border border-border/50"
                                                >
                                                    <Activity className="w-4 h-4 mt-1 text-muted-foreground" />
                                                    <div className="flex-1">
                                                        <div className="flex items-center gap-2">
                                                            <span className="font-medium">{activity.user_id}</span>
                                                            <Badge variant="outline" className="text-xs">
                                                                {activity.action}
                                                            </Badge>
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
                    </Tabs>
                </motion.div>
            </main>
        </div>
    );
}
