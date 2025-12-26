import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';
import { Loader2 } from 'lucide-react';

interface AdminProtectedRouteProps {
    children: React.ReactNode;
}

export function AdminProtectedRoute({ children }: AdminProtectedRouteProps) {
    const { user, isAdmin, isLoading } = useAuth();
    const location = useLocation();

    if (isLoading) {
        return (
            <div className="min-h-screen flex items-center justify-center bg-background">
                <Loader2 className="w-8 h-8 animate-spin text-primary" />
            </div>
        );
    }

    if (!user) {
        // Not logged in, redirect to auth
        return <Navigate to="/auth" state={{ from: location }} replace />;
    }

    if (!isAdmin) {
        // Logged in but not admin, redirect to dashboard
        return <Navigate to="/dashboard" replace />;
    }

    return <>{children}</>;
}
