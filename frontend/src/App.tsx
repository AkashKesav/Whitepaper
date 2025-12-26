import { Toaster } from "@/components/ui/toaster";
import { TooltipProvider } from "@/components/ui/tooltip";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { AuthProvider } from "@/contexts/AuthContext";
import { AdminProtectedRoute } from "@/components/AdminProtectedRoute";
import Index from "./pages/Index";
import Auth from "./pages/Auth";
import { Dashboard } from "./pages/Dashboard";
import Admin from "./pages/Admin";
import Chat from "./pages/Chat";
import Groups from "./pages/Groups";
import Settings from "./pages/Settings";
import Ingestion from "./pages/Ingestion";

const queryClient = new QueryClient();

const App = () => (
  <QueryClientProvider client={queryClient}>
    <AuthProvider>
      <TooltipProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/" element={<Index />} />
            <Route path="/login" element={<Auth />} />
            <Route path="/auth" element={<Auth />} />
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/chat" element={<Chat />} />
            <Route path="/groups" element={<Groups />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="/ingestion" element={<Ingestion />} />
            <Route
              path="/admin"
              element={
                <AdminProtectedRoute>
                  <Admin />
                </AdminProtectedRoute>
              }
            />
            <Route path="*" element={<Index />} />
          </Routes>
        </BrowserRouter>
        <Toaster />
      </TooltipProvider>
    </AuthProvider>
  </QueryClientProvider>
);

export default App;
