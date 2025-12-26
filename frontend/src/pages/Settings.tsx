import React, { useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { cn } from '@/lib/utils';
import {
    Settings as SettingsIcon, User, Bell, Key, Palette,
    Shield, Database, ChevronRight, Moon, Sun, Check,
    LogOut, Trash2, Download, Upload
} from 'lucide-react';

type SettingsSection = 'profile' | 'notifications' | 'api' | 'appearance' | 'privacy' | 'data';

export const Settings: React.FC = () => {
    const { user, logout } = useAuth();
    const [activeSection, setActiveSection] = useState<SettingsSection>('profile');
    const [theme, setTheme] = useState<'dark' | 'light' | 'system'>('dark');
    const [notifications, setNotifications] = useState({
        email: true,
        push: false,
        insights: true,
        updates: true,
    });

    const sections = [
        { id: 'profile' as const, label: 'Profile', icon: User },
        { id: 'notifications' as const, label: 'Notifications', icon: Bell },
        { id: 'api' as const, label: 'API Keys', icon: Key },
        { id: 'appearance' as const, label: 'Appearance', icon: Palette },
        { id: 'privacy' as const, label: 'Privacy & Security', icon: Shield },
        { id: 'data' as const, label: 'Data Management', icon: Database },
    ];

    return (
        <div className="h-screen flex bg-[#1C1C1E]">
            {/* Sidebar */}
            <div
                className="w-64 flex flex-col border-r"
                style={{ borderColor: 'rgba(255,255,255,0.1)' }}
            >
                <div className="p-6 border-b border-white/10">
                    <h1 className="text-lg font-semibold text-white flex items-center gap-2">
                        <SettingsIcon className="w-5 h-5" />
                        Settings
                    </h1>
                </div>

                <div className="flex-1 p-3">
                    {sections.map((section) => (
                        <button
                            key={section.id}
                            onClick={() => setActiveSection(section.id)}
                            className={cn(
                                "w-full flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm transition-colors mb-1",
                                activeSection === section.id
                                    ? "bg-white/10 text-white"
                                    : "text-white/60 hover:bg-white/5 hover:text-white"
                            )}
                        >
                            <section.icon className="w-4 h-4" />
                            {section.label}
                        </button>
                    ))}
                </div>

                <div className="p-3 border-t border-white/10">
                    <button
                        onClick={() => logout()}
                        className="w-full flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm text-red-400 hover:bg-red-500/10 transition-colors"
                    >
                        <LogOut className="w-4 h-4" />
                        Sign Out
                    </button>
                </div>
            </div>

            {/* Main Content */}
            <div className="flex-1 overflow-y-auto bg-black">
                <div className="max-w-2xl mx-auto p-8">
                    {activeSection === 'profile' && (
                        <div className="space-y-8">
                            <div>
                                <h2 className="text-xl font-semibold text-white mb-6">Profile</h2>

                                <div className="flex items-center gap-6 mb-8">
                                    <div className="w-20 h-20 rounded-full bg-gradient-to-br from-purple-500 to-blue-500 flex items-center justify-center text-2xl text-white font-semibold">
                                        {user?.username?.charAt(0).toUpperCase() || 'U'}
                                    </div>
                                    <div>
                                        <button className="px-4 py-2 bg-white/10 hover:bg-white/15 rounded-lg text-sm text-white transition-colors">
                                            Change Photo
                                        </button>
                                    </div>
                                </div>

                                <div className="space-y-4">
                                    <div>
                                        <label className="block text-sm text-white/50 mb-2">Username</label>
                                        <input
                                            type="text"
                                            defaultValue={user?.username || ''}
                                            className="w-full px-4 py-3 bg-white/5 border border-white/10 rounded-xl text-white outline-none focus:border-white/30 transition-colors"
                                        />
                                    </div>
                                    <div>
                                        <label className="block text-sm text-white/50 mb-2">Email</label>
                                        <input
                                            type="email"
                                            defaultValue="user@example.com"
                                            className="w-full px-4 py-3 bg-white/5 border border-white/10 rounded-xl text-white outline-none focus:border-white/30 transition-colors"
                                        />
                                    </div>
                                    <button className="px-6 py-2.5 bg-blue-500 hover:bg-blue-600 rounded-xl text-sm text-white font-medium transition-colors">
                                        Save Changes
                                    </button>
                                </div>
                            </div>
                        </div>
                    )}

                    {activeSection === 'notifications' && (
                        <div>
                            <h2 className="text-xl font-semibold text-white mb-6">Notifications</h2>
                            <div className="space-y-4">
                                {[
                                    { key: 'email', label: 'Email Notifications', desc: 'Receive updates via email' },
                                    { key: 'push', label: 'Push Notifications', desc: 'Browser push notifications' },
                                    { key: 'insights', label: 'Insight Alerts', desc: 'Get notified about new insights' },
                                    { key: 'updates', label: 'Product Updates', desc: 'News about new features' },
                                ].map((item) => (
                                    <div
                                        key={item.key}
                                        className="flex items-center justify-between p-4 bg-white/5 rounded-xl"
                                    >
                                        <div>
                                            <p className="text-sm font-medium text-white">{item.label}</p>
                                            <p className="text-xs text-white/50 mt-0.5">{item.desc}</p>
                                        </div>
                                        <button
                                            onClick={() => setNotifications(prev => ({
                                                ...prev,
                                                [item.key]: !prev[item.key as keyof typeof notifications]
                                            }))}
                                            className={cn(
                                                "w-12 h-7 rounded-full transition-colors relative",
                                                notifications[item.key as keyof typeof notifications]
                                                    ? "bg-blue-500"
                                                    : "bg-white/20"
                                            )}
                                        >
                                            <span
                                                className={cn(
                                                    "absolute top-1 w-5 h-5 rounded-full bg-white transition-all",
                                                    notifications[item.key as keyof typeof notifications]
                                                        ? "left-6"
                                                        : "left-1"
                                                )}
                                            />
                                        </button>
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}

                    {activeSection === 'api' && (
                        <div>
                            <h2 className="text-xl font-semibold text-white mb-6">API Keys</h2>
                            <p className="text-sm text-white/50 mb-6">
                                Manage API keys for external integrations with the Memory Kernel.
                            </p>
                            <div className="space-y-4">
                                <div className="p-4 bg-white/5 rounded-xl">
                                    <div className="flex items-center justify-between mb-2">
                                        <span className="text-sm font-medium text-white">Production Key</span>
                                        <span className="text-xs text-green-400 bg-green-500/20 px-2 py-0.5 rounded-full">Active</span>
                                    </div>
                                    <code className="text-xs text-white/50 font-mono">mk_prod_****************************</code>
                                </div>
                                <button className="w-full p-4 border border-dashed border-white/20 rounded-xl text-white/50 hover:bg-white/5 hover:border-white/30 transition-colors text-sm">
                                    + Generate New API Key
                                </button>
                            </div>
                        </div>
                    )}

                    {activeSection === 'appearance' && (
                        <div>
                            <h2 className="text-xl font-semibold text-white mb-6">Appearance</h2>
                            <div className="space-y-4">
                                <p className="text-sm text-white/50 mb-4">Choose your preferred theme</p>
                                <div className="grid grid-cols-3 gap-4">
                                    {[
                                        { id: 'dark', label: 'Dark', icon: Moon },
                                        { id: 'light', label: 'Light', icon: Sun },
                                        { id: 'system', label: 'System', icon: SettingsIcon },
                                    ].map((t) => (
                                        <button
                                            key={t.id}
                                            onClick={() => setTheme(t.id as typeof theme)}
                                            className={cn(
                                                "p-4 rounded-xl border transition-all text-center",
                                                theme === t.id
                                                    ? "border-blue-500 bg-blue-500/10"
                                                    : "border-white/10 hover:border-white/30"
                                            )}
                                        >
                                            <t.icon className={cn(
                                                "w-6 h-6 mx-auto mb-2",
                                                theme === t.id ? "text-blue-400" : "text-white/50"
                                            )} />
                                            <span className={cn(
                                                "text-sm",
                                                theme === t.id ? "text-white" : "text-white/60"
                                            )}>
                                                {t.label}
                                            </span>
                                        </button>
                                    ))}
                                </div>
                            </div>
                        </div>
                    )}

                    {activeSection === 'privacy' && (
                        <div>
                            <h2 className="text-xl font-semibold text-white mb-6">Privacy & Security</h2>
                            <div className="space-y-4">
                                <div className="p-4 bg-white/5 rounded-xl">
                                    <h3 className="text-sm font-medium text-white mb-2">Two-Factor Authentication</h3>
                                    <p className="text-xs text-white/50 mb-3">Add an extra layer of security to your account</p>
                                    <button className="px-4 py-2 bg-white/10 hover:bg-white/15 rounded-lg text-sm text-white transition-colors">
                                        Enable 2FA
                                    </button>
                                </div>
                                <div className="p-4 bg-white/5 rounded-xl">
                                    <h3 className="text-sm font-medium text-white mb-2">Change Password</h3>
                                    <p className="text-xs text-white/50 mb-3">Update your account password</p>
                                    <button className="px-4 py-2 bg-white/10 hover:bg-white/15 rounded-lg text-sm text-white transition-colors">
                                        Change Password
                                    </button>
                                </div>
                            </div>
                        </div>
                    )}

                    {activeSection === 'data' && (
                        <div>
                            <h2 className="text-xl font-semibold text-white mb-6">Data Management</h2>
                            <div className="space-y-4">
                                <div className="p-4 bg-white/5 rounded-xl flex items-center justify-between">
                                    <div>
                                        <h3 className="text-sm font-medium text-white">Export Data</h3>
                                        <p className="text-xs text-white/50 mt-0.5">Download all your memory data</p>
                                    </div>
                                    <button className="px-4 py-2 bg-white/10 hover:bg-white/15 rounded-lg text-sm text-white transition-colors flex items-center gap-2">
                                        <Download className="w-4 h-4" />
                                        Export
                                    </button>
                                </div>
                                <div className="p-4 bg-red-500/10 border border-red-500/20 rounded-xl flex items-center justify-between">
                                    <div>
                                        <h3 className="text-sm font-medium text-red-400">Delete Account</h3>
                                        <p className="text-xs text-white/50 mt-0.5">Permanently remove your account and data</p>
                                    </div>
                                    <button className="px-4 py-2 bg-red-500/20 hover:bg-red-500/30 rounded-lg text-sm text-red-400 transition-colors flex items-center gap-2">
                                        <Trash2 className="w-4 h-4" />
                                        Delete
                                    </button>
                                </div>
                            </div>
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
};

export default Settings;
