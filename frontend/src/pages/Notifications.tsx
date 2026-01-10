import React, { useState, useEffect } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { api } from '@/lib/api';
import { cn } from '@/lib/utils';
import {
    Bell, Check, X, Users, Mail, Clock, 
    RefreshCw, Inbox, ChevronRight
} from 'lucide-react';

interface Invitation {
    uid: string;
    workspace_ns: string;
    invitee_user_id: string;
    invited_by: string;
    role: string;
    status: string;
    created_at: string;
    expires_at?: string;
}

export const Notifications: React.FC = () => {
    const { user } = useAuth();
    const [invitations, setInvitations] = useState<Invitation[]>([]);
    const [loading, setLoading] = useState(true);
    const [processing, setProcessing] = useState<string | null>(null);

    const fetchInvitations = async () => {
        setLoading(true);
        try {
            const data = await api.getPendingInvitations();
            setInvitations(data);
        } catch (e) {
            console.error("Failed to fetch invitations", e);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchInvitations();
        // Poll every 30 seconds for new invitations
        const interval = setInterval(fetchInvitations, 30000);
        return () => clearInterval(interval);
    }, [user]);

    const handleAccept = async (invitationId: string) => {
        setProcessing(invitationId);
        try {
            await api.acceptInvitation(invitationId);
            // Remove from list after accepting
            setInvitations(prev => prev.filter(inv => inv.uid !== invitationId));
        } catch (e) {
            alert("Failed to accept invitation");
        } finally {
            setProcessing(null);
        }
    };

    const handleDecline = async (invitationId: string) => {
        setProcessing(invitationId);
        try {
            await api.declineInvitation(invitationId);
            // Remove from list after declining
            setInvitations(prev => prev.filter(inv => inv.uid !== invitationId));
        } catch (e) {
            alert("Failed to decline invitation");
        } finally {
            setProcessing(null);
        }
    };

    const formatDate = (dateStr: string) => {
        try {
            return new Date(dateStr).toLocaleDateString('en-US', {
                month: 'short',
                day: 'numeric',
                hour: '2-digit',
                minute: '2-digit'
            });
        } catch {
            return dateStr;
        }
    };

    const pendingCount = invitations.filter(inv => inv.status === 'pending').length;

    return (
        <div className="min-h-screen bg-black p-8">
            <div className="max-w-3xl mx-auto">
                {/* Header */}
                <div className="flex items-center justify-between mb-8">
                    <div className="flex items-center gap-4">
                        <div className="w-12 h-12 rounded-2xl bg-gradient-to-br from-orange-500 to-red-500 flex items-center justify-center">
                            <Bell className="w-6 h-6 text-white" />
                        </div>
                        <div>
                            <h1 className="text-2xl font-semibold text-white">Notifications</h1>
                            <p className="text-white/50 text-sm">
                                {pendingCount > 0 
                                    ? `${pendingCount} pending invitation${pendingCount !== 1 ? 's' : ''}`
                                    : 'No pending notifications'
                                }
                            </p>
                        </div>
                    </div>
                    <button
                        onClick={fetchInvitations}
                        disabled={loading}
                        className="p-2 hover:bg-white/10 rounded-lg transition-colors disabled:opacity-50"
                        title="Refresh"
                    >
                        <RefreshCw className={cn("w-5 h-5 text-white/60", loading && "animate-spin")} />
                    </button>
                </div>

                {/* Invitations List */}
                <div className="space-y-4">
                    {loading && invitations.length === 0 ? (
                        <div className="text-center py-16 text-white/40">
                            <RefreshCw className="w-8 h-8 mx-auto mb-4 animate-spin" />
                            <p>Loading notifications...</p>
                        </div>
                    ) : invitations.length === 0 ? (
                        <div className="text-center py-16">
                            <Inbox className="w-16 h-16 text-white/20 mx-auto mb-4" />
                            <h3 className="text-lg font-medium text-white/60 mb-2">All caught up!</h3>
                            <p className="text-white/40 text-sm">
                                You don't have any pending invitations.
                            </p>
                        </div>
                    ) : (
                        invitations.map((invitation) => (
                            <div
                                key={invitation.uid}
                                className={cn(
                                    "bg-white/5 rounded-2xl p-6 border transition-all",
                                    invitation.status === 'pending' 
                                        ? "border-orange-500/30 hover:border-orange-500/50" 
                                        : "border-white/10 opacity-60"
                                )}
                            >
                                <div className="flex items-start justify-between gap-4">
                                    <div className="flex items-start gap-4">
                                        <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-blue-500 to-purple-500 flex items-center justify-center flex-shrink-0">
                                            <Users className="w-6 h-6 text-white" />
                                        </div>
                                        <div>
                                            <h3 className="text-lg font-medium text-white">
                                                Workspace Invitation
                                            </h3>
                                            <p className="text-sm text-white/60 mt-1">
                                                <span className="text-blue-400 font-medium">{invitation.invited_by}</span>
                                                {' '}invited you to join{' '}
                                                <span className="text-purple-400 font-medium">{invitation.workspace_ns}</span>
                                            </p>
                                            <div className="flex items-center gap-4 mt-3">
                                                <span className="flex items-center gap-1 text-xs text-white/40">
                                                    <Mail className="w-3 h-3" />
                                                    Role: {invitation.role || 'member'}
                                                </span>
                                                <span className="flex items-center gap-1 text-xs text-white/40">
                                                    <Clock className="w-3 h-3" />
                                                    {formatDate(invitation.created_at)}
                                                </span>
                                            </div>
                                        </div>
                                    </div>

                                    {invitation.status === 'pending' && (
                                        <div className="flex gap-2 flex-shrink-0">
                                            <button
                                                onClick={() => handleDecline(invitation.uid)}
                                                disabled={processing === invitation.uid}
                                                className="p-2 hover:bg-red-500/20 rounded-lg transition-colors disabled:opacity-50"
                                                title="Decline"
                                            >
                                                <X className="w-5 h-5 text-red-400" />
                                            </button>
                                            <button
                                                onClick={() => handleAccept(invitation.uid)}
                                                disabled={processing === invitation.uid}
                                                className="flex items-center gap-2 px-4 py-2 bg-green-500 hover:bg-green-600 disabled:opacity-50 rounded-lg text-sm text-white transition-colors"
                                            >
                                                <Check className="w-4 h-4" />
                                                Accept
                                            </button>
                                        </div>
                                    )}

                                    {invitation.status === 'accepted' && (
                                        <span className="px-3 py-1 bg-green-500/20 text-green-400 rounded-full text-xs">
                                            Accepted
                                        </span>
                                    )}

                                    {invitation.status === 'declined' && (
                                        <span className="px-3 py-1 bg-red-500/20 text-red-400 rounded-full text-xs">
                                            Declined
                                        </span>
                                    )}
                                </div>
                            </div>
                        ))
                    )}
                </div>

                {/* Info Box */}
                <div className="mt-8 p-4 bg-blue-500/10 border border-blue-500/30 rounded-xl">
                    <div className="flex items-start gap-3">
                        <Bell className="w-5 h-5 text-blue-400 flex-shrink-0 mt-0.5" />
                        <div>
                            <h4 className="text-sm font-medium text-blue-400">About Invitations</h4>
                            <p className="text-xs text-white/50 mt-1">
                                When someone invites you to a group or workspace, you'll see the invitation here.
                                Accept to join and share memories within that group context.
                            </p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default Notifications;
