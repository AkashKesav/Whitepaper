import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';

interface User {
    username: string;
    role: 'admin' | 'user';
    token: string;
}

interface AuthContextType {
    user: User | null;
    isAdmin: boolean;
    isLoading: boolean;
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
            }
        }
        setIsLoading(false);
    }, []);

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
                return { success: true };
            }

            return { success: false, error: 'Invalid token received' };
        } catch (error) {
            return { success: false, error: 'Network error. Please try again.' };
        }
    }, []);

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
                return { success: true };
            }

            return { success: false, error: 'Invalid token received' };
        } catch (error) {
            return { success: false, error: 'Network error. Please try again.' };
        }
    }, []);

    const logout = useCallback(() => {
        setUser(null);
        localStorage.removeItem('rmk_token');
    }, []);

    const isAdmin = user?.role === 'admin';

    return (
        <AuthContext.Provider value={{ user, isAdmin, isLoading, login, register, logout }}>
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
