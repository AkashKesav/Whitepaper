import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';

interface User {
    username: string;
    role: 'admin' | 'user';
    token: string;
}

interface UserPreferences {
    nimApiKey: string;  // Local temporary storage before save
    openaiApiKey: string;  // Local temporary storage before save
    anthropicApiKey: string;  // Local temporary storage before save
    hasNimKey: boolean;  // Backend status
    hasOpenaiKey: boolean;  // Backend status
    hasAnthropicKey: boolean;  // Backend status
    theme: 'dark' | 'light' | 'system';
    notifications: boolean;
}

interface AuthContextType {
    user: User | null;
    isAdmin: boolean;
    isLoading: boolean;
    preferences: UserPreferences;
    updatePreference: (key: keyof UserPreferences, value: string) => void;
    saveAPIKey: (provider: 'nim' | 'openai' | 'anthropic', key: string) => Promise<{ success: boolean; error?: string }>;
    deleteAPIKey: (provider: 'nim' | 'openai' | 'anthropic') => Promise<{ success: boolean; error?: string }>;
    loadPreferences: () => Promise<void>;
    login: (username: string, password: string) => Promise<{ success: boolean; error?: string }>;
    register: (username: string, password: string, role: 'admin' | 'user') => Promise<{ success: boolean; error?: string }>;
    logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

const API_BASE_URL = '';

// Decode JWT payload without verification (for role extraction)
function decodeJWT(token: string): { sub: string; role: string; exp: number } | null {
    try {
        const payload = token.split('.')[1];
        const decoded = JSON.parse(atob(payload));
        return decoded;
    } catch {
        return null;
    }
}

// Check if token is expired
function isTokenExpired(token: string): boolean {
    const decoded = decodeJWT(token);
    if (!decoded) return true;
    return decoded.exp * 1000 < Date.now();
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
    const [user, setUser] = useState<User | null>(null);
    const [isLoading, setIsLoading] = useState(true);

    // Initialize preferences from localStorage (for theme/notifications only)
    // API keys are loaded from backend
    const [preferences, setPreferences] = useState<UserPreferences>(() => ({
        nimApiKey: '',  // Local temporary storage before save
        openaiApiKey: '',  // Local temporary storage before save
        anthropicApiKey: '',  // Local temporary storage before save
        hasNimKey: false,  // Backend status
        hasOpenaiKey: false,  // Backend status
        hasAnthropicKey: false,  // Backend status
        theme: (localStorage.getItem('rmk_theme') as 'dark' | 'light' | 'system') || 'dark',
        notifications: localStorage.getItem('rmk_notifications') === 'true',
    }));

    // Update preference and persist to localStorage (for theme/notifications)
    const updatePreference = useCallback((key: keyof UserPreferences, value: string | boolean) => {
        setPreferences(prev => ({ ...prev, [key]: value }));
        if (key === 'theme' || key === 'notifications') {
            localStorage.setItem(`rmk_${key}`, String(value));
        }
    }, []);

    // Load preferences from backend after login
    const loadPreferences = useCallback(async () => {
        const token = localStorage.getItem('rmk_token');
        if (!token) return;

        try {
            const response = await fetch(`${API_BASE_URL}/api/user/settings`, {
                headers: { 'Authorization': `Bearer ${token}` }
            });

            if (response.ok) {
                const data = await response.json();
                setPreferences(prev => ({
                    ...prev,
                    nimApiKey: '',  // Never load actual keys, only status
                    openaiApiKey: '',
                    anthropicApiKey: '',
                    hasNimKey: data.has_nim_key || false,
                    hasOpenaiKey: data.has_openai_key || false,
                    hasAnthropicKey: data.has_anthropic_key || false,
                    theme: data.theme || prev.theme,
                    notifications: data.notifications_enabled ?? prev.notifications,
                }));
            }
        } catch (error) {
            console.warn('Failed to load preferences from backend:', error);
        }
    }, []);

    // Save API key to backend (encrypted)
    const saveAPIKey = useCallback(async (provider: 'nim' | 'openai' | 'anthropic', key: string): Promise<{ success: boolean; error?: string }> => {
        const token = localStorage.getItem('rmk_token');
        if (!token) return { success: false, error: 'Not authenticated' };

        try {
            const body: Record<string, string> = {};
            if (provider === 'nim') body.nim_api_key = key;
            if (provider === 'openai') body.openai_api_key = key;
            if (provider === 'anthropic') body.anthropic_api_key = key;

            const response = await fetch(`${API_BASE_URL}/api/user/settings`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify(body),
            });

            if (response.ok) {
                // Update local status
                await loadPreferences();
                return { success: true };
            } else {
                const errorText = await response.text();
                return { success: false, error: errorText || 'Failed to save API key' };
            }
        } catch (error) {
            return { success: false, error: 'Network error. Please try again.' };
        }
    }, [loadPreferences]);

    // Delete API key from backend
    const deleteAPIKey = useCallback(async (provider: 'nim' | 'openai' | 'anthropic'): Promise<{ success: boolean; error?: string }> => {
        const token = localStorage.getItem('rmk_token');
        if (!token) return { success: false, error: 'Not authenticated' };

        try {
            const response = await fetch(`${API_BASE_URL}/api/user/settings/keys/${provider}`, {
                method: 'DELETE',
                headers: { 'Authorization': `Bearer ${token}` }
            });

            if (response.ok) {
                // Update local status
                await loadPreferences();
                return { success: true };
            } else {
                const errorText = await response.text();
                return { success: false, error: errorText || 'Failed to delete API key' };
            }
        } catch (error) {
            return { success: false, error: 'Network error. Please try again.' };
        }
    }, [loadPreferences]);

    // Initialize from localStorage
    useEffect(() => {
        const storedToken = localStorage.getItem('rmk_token');
        if (storedToken && !isTokenExpired(storedToken)) {
            const decoded = decodeJWT(storedToken);
            if (decoded) {
                setUser({
                    username: decoded.sub,
                    role: (decoded.role as 'admin' | 'user') || 'user',
                    token: storedToken,
                });
                // Load preferences from backend after setting user
                loadPreferences();
            }
        }
        setIsLoading(false);
    }, [loadPreferences]);

    const login = useCallback(async (username: string, password: string): Promise<{ success: boolean; error?: string }> => {
        try {
            const response = await fetch(`${API_BASE_URL}/api/login`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password }),
            });

            if (!response.ok) {
                const errorText = await response.text();
                return { success: false, error: errorText || 'Login failed' };
            }

            const data = await response.json();
            const decoded = decodeJWT(data.token);

            if (decoded) {
                const newUser: User = {
                    username: decoded.sub,
                    role: (decoded.role as 'admin' | 'user') || 'user',
                    token: data.token,
                };
                setUser(newUser);
                localStorage.setItem('rmk_token', data.token);
                // Load preferences from backend after login
                await loadPreferences();
                return { success: true };
            }

            return { success: false, error: 'Invalid token received' };
        } catch (error) {
            return { success: false, error: 'Network error. Please try again.' };
        }
    }, [loadPreferences]);

    const register = useCallback(async (username: string, password: string, role: 'admin' | 'user'): Promise<{ success: boolean; error?: string }> => {
        try {
            const response = await fetch(`${API_BASE_URL}/api/register`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password, role }),
            });

            if (!response.ok) {
                const errorText = await response.text();
                return { success: false, error: errorText || 'Registration failed' };
            }

            const data = await response.json();
            const decoded = decodeJWT(data.token);

            if (decoded) {
                const newUser: User = {
                    username: decoded.sub,
                    role: (decoded.role as 'admin' | 'user') || 'user',
                    token: data.token,
                };
                setUser(newUser);
                localStorage.setItem('rmk_token', data.token);
                // Load preferences from backend after registration
                await loadPreferences();
                return { success: true };
            }

            return { success: false, error: 'Invalid token received' };
        } catch (error) {
            return { success: false, error: 'Network error. Please try again.' };
        }
    }, [loadPreferences]);

    const logout = useCallback(() => {
        setUser(null);
        localStorage.removeItem('rmk_token');
        // Clear API keys from state
        setPreferences(prev => ({
            ...prev,
            nimApiKey: '',
            openaiApiKey: '',
            anthropicApiKey: '',
            hasNimKey: false,
            hasOpenaiKey: false,
            hasAnthropicKey: false,
        }));
    }, []);

    const isAdmin = user?.role === 'admin';

    return (
        <AuthContext.Provider value={{ user, isAdmin, isLoading, preferences, updatePreference, saveAPIKey, deleteAPIKey, loadPreferences, login, register, logout }}>
            {children}
        </AuthContext.Provider>
    );
}

export function useAuth() {
    const context = useContext(AuthContext);
    if (context === undefined) {
        throw new Error('useAuth must be used within an AuthProvider');
    }
    return context;
}

// Helper hook for admin-only checks
export function useRequireAdmin() {
    const { isAdmin, isLoading } = useAuth();
    return { isAdmin, isLoading };
}
