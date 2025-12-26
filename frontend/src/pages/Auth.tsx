import { motion, AnimatePresence } from 'framer-motion';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ArrowLeft, Loader2, Eye, EyeOff, Shield } from 'lucide-react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useState, useEffect } from 'react';
import { z } from 'zod';
import { useToast } from '@/hooks/use-toast';
import { useAuth } from '@/contexts/AuthContext';
import { AbstractArt } from '@/components/auth/AbstractArt';

// Validation schemas
const usernameSchema = z.string().min(3, { message: "Username must be at least 3 characters" });
const passwordSchema = z.string().min(6, { message: "Password must be at least 6 characters" });

const signUpSchema = z.object({
  username: usernameSchema,
  password: passwordSchema,
  confirmPassword: z.string(),
}).refine((data) => data.password === data.confirmPassword, {
  message: "Passwords don't match",
  path: ["confirmPassword"],
});

const signInSchema = z.object({
  username: usernameSchema,
  password: z.string().min(1, { message: "Password is required" }),
});

const Auth = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { toast } = useToast();
  const { user, login, register, isLoading: authLoading } = useAuth();

  const [isSignUp, setIsSignUp] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [showIntro, setShowIntro] = useState(false);
  const [introComplete, setIntroComplete] = useState(false);

  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [role, setRole] = useState<'user' | 'admin'>('user');
  const [errors, setErrors] = useState<Record<string, string>>({});

  // Check if we came from the landing page transition
  useEffect(() => {
    const fromLanding = location.state?.fromLanding;
    if (fromLanding) {
      setShowIntro(true);
      setTimeout(() => {
        setIntroComplete(true);
      }, 1200);
    } else {
      setIntroComplete(true);
    }
  }, [location]);

  // Redirect if already logged in
  useEffect(() => {
    if (user && !authLoading) {
      navigate('/dashboard');
    }
  }, [user, authLoading, navigate]);

  const validateForm = () => {
    const schema = isSignUp ? signUpSchema : signInSchema;
    const data = isSignUp
      ? { username, password, confirmPassword }
      : { username, password };

    const result = schema.safeParse(data);

    if (!result.success) {
      const formattedErrors: Record<string, string> = {};
      result.error.errors.forEach((err) => {
        if (err.path[0]) {
          formattedErrors[err.path[0] as string] = err.message;
        }
      });
      setErrors(formattedErrors);
      return false;
    }

    setErrors({});
    return true;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateForm()) return;

    setIsLoading(true);

    try {
      if (isSignUp) {
        const result = await register(username, password, role);

        if (!result.success) {
          toast({
            variant: "destructive",
            title: "Sign up failed",
            description: result.error || "Failed to create account",
          });
        } else {
          toast({
            title: "Account created!",
            description: `Welcome ${username}! You are now logged in as ${role}.`,
          });
          navigate('/dashboard');
        }
      } else {
        const result = await login(username, password);

        if (!result.success) {
          toast({
            variant: "destructive",
            title: "Login failed",
            description: result.error || "Invalid credentials",
          });
        } else {
          toast({
            title: "Welcome back!",
            description: `Logged in as ${username}`,
          });
          navigate('/dashboard');
        }
      }
    } catch (error) {
      toast({
        variant: "destructive",
        title: "Error",
        description: "An unexpected error occurred. Please try again.",
      });
    } finally {
      setIsLoading(false);
    }
  };

  const toggleMode = () => {
    setIsSignUp(!isSignUp);
    setErrors({});
    setPassword('');
    setConfirmPassword('');
    setRole('user');
  };

  return (
    <div className="min-h-screen bg-background flex relative overflow-hidden">
      {/* Intro animation */}
      <AnimatePresence>
        {showIntro && !introComplete && (
          <motion.div
            className="fixed inset-0 z-50 bg-background flex items-center justify-center"
            initial={{ opacity: 1 }}
            animate={{ opacity: 0 }}
            transition={{ duration: 0.8, delay: 0.3, ease: [0.4, 0, 0.2, 1] }}
          >
            <motion.div
              className="absolute rounded-full"
              style={{
                background: 'radial-gradient(circle, hsl(var(--primary) / 0.15) 0%, transparent 70%)',
                width: '100vw',
                height: '100vw',
              }}
              initial={{ opacity: 0.6, scale: 1.5 }}
              animate={{ opacity: 0, scale: 0.5 }}
              transition={{ duration: 1, ease: [0.4, 0, 0.2, 1] }}
            />
          </motion.div>
        )}
      </AnimatePresence>

      {/* Left side - Abstract Art (hidden on mobile) */}
      <motion.div
        className="hidden lg:block lg:w-1/2 xl:w-[55%] relative"
        initial={{ opacity: 0, x: -50 }}
        animate={{ opacity: introComplete ? 1 : 0, x: introComplete ? 0 : -50 }}
        transition={{ duration: 0.8, delay: 0.2 }}
      >
        <AbstractArt />
      </motion.div>

      {/* Right side - Auth form */}
      <motion.div
        className="w-full lg:w-1/2 xl:w-[45%] flex items-center justify-center p-6 lg:p-12"
        initial={{ opacity: 0, x: 50 }}
        animate={{ opacity: introComplete ? 1 : 0, x: introComplete ? 0 : 50 }}
        transition={{ duration: 0.8, delay: 0.3 }}
      >
        <div className="w-full max-w-md">
          {/* Back button */}
          <button
            onClick={() => navigate('/')}
            className="flex items-center gap-2 text-muted-foreground hover:text-foreground transition-colors mb-8 text-sm"
          >
            <ArrowLeft className="w-4 h-4" />
            Back to home
          </button>

          {/* Mobile logo */}
          <div className="lg:hidden flex items-center gap-3 mb-8">
            <div className="w-10 h-10 bg-primary rounded-xl flex items-center justify-center">
              <div className="w-5 h-5 bg-primary-foreground rounded-md" />
            </div>
            <span className="text-xl font-semibold text-foreground">MemoryKernel</span>
          </div>

          {/* Header */}
          <div className="mb-8">
            <AnimatePresence mode="wait">
              <motion.div
                key={isSignUp ? 'signup' : 'signin'}
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -10 }}
                transition={{ duration: 0.2 }}
              >
                <h1 className="text-3xl font-semibold text-foreground mb-2">
                  {isSignUp ? 'Create an account' : 'Welcome back'}
                </h1>
                <p className="text-muted-foreground">
                  {isSignUp
                    ? 'Sign up to get started with your account'
                    : 'Sign in to your account to continue'}
                </p>
              </motion.div>
            </AnimatePresence>
          </div>

          {/* Form */}
          <form onSubmit={handleSubmit} className="space-y-5">
            <div className="space-y-2">
              <label className="text-sm font-medium text-foreground">Username</label>
              <Input
                type="text"
                placeholder="johndoe"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className={`h-12 bg-muted/50 border-border ${errors.username ? 'border-destructive' : ''}`}
                disabled={isLoading}
              />
              {errors.username && (
                <p className="text-xs text-destructive">{errors.username}</p>
              )}
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium text-foreground">Password</label>
              <div className="relative">
                <Input
                  type={showPassword ? 'text' : 'password'}
                  placeholder="••••••••"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className={`h-12 bg-muted/50 border-border pr-10 ${errors.password ? 'border-destructive' : ''}`}
                  disabled={isLoading}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                >
                  {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
              {errors.password && (
                <p className="text-xs text-destructive">{errors.password}</p>
              )}
            </div>

            <AnimatePresence>
              {isSignUp && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  transition={{ duration: 0.2 }}
                  className="space-y-4 overflow-hidden"
                >
                  {/* Confirm Password */}
                  <div className="space-y-2">
                    <label className="text-sm font-medium text-foreground">Confirm Password</label>
                    <div className="relative">
                      <Input
                        type={showConfirmPassword ? 'text' : 'password'}
                        placeholder="••••••••"
                        value={confirmPassword}
                        onChange={(e) => setConfirmPassword(e.target.value)}
                        className={`h-12 bg-muted/50 border-border pr-10 ${errors.confirmPassword ? 'border-destructive' : ''}`}
                        disabled={isLoading}
                      />
                      <button
                        type="button"
                        onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                        className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                      >
                        {showConfirmPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                      </button>
                    </div>
                    {errors.confirmPassword && (
                      <p className="text-xs text-destructive">{errors.confirmPassword}</p>
                    )}
                  </div>

                  {/* Role Selection */}
                  <div className="space-y-2">
                    <label className="text-sm font-medium text-foreground">Account Type</label>
                    <div className="grid grid-cols-2 gap-3">
                      <button
                        type="button"
                        onClick={() => setRole('user')}
                        className={`p-3 rounded-lg border-2 transition-all ${role === 'user'
                            ? 'border-primary bg-primary/10'
                            : 'border-border hover:border-primary/50'
                          }`}
                      >
                        <div className="text-sm font-medium">User</div>
                        <div className="text-xs text-muted-foreground">Regular access</div>
                      </button>
                      <button
                        type="button"
                        onClick={() => setRole('admin')}
                        className={`p-3 rounded-lg border-2 transition-all ${role === 'admin'
                            ? 'border-primary bg-primary/10'
                            : 'border-border hover:border-primary/50'
                          }`}
                      >
                        <div className="text-sm font-medium flex items-center justify-center gap-1">
                          <Shield className="w-3 h-3" /> Admin
                        </div>
                        <div className="text-xs text-muted-foreground">Full access</div>
                      </button>
                    </div>
                  </div>
                </motion.div>
              )}
            </AnimatePresence>

            <Button
              type="submit"
              className="w-full h-12 mt-2"
              size="lg"
              disabled={isLoading}
            >
              {isLoading ? (
                <>
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                  {isSignUp ? 'Creating account...' : 'Signing in...'}
                </>
              ) : (
                isSignUp ? 'Create account' : 'Sign in'
              )}
            </Button>
          </form>

          {/* Footer */}
          <div className="mt-6 text-center text-sm text-muted-foreground">
            {isSignUp ? 'Already have an account?' : "Don't have an account?"}{' '}
            <button
              onClick={toggleMode}
              className="text-primary hover:underline font-medium"
              disabled={isLoading}
            >
              {isSignUp ? 'Sign in' : 'Sign up'}
            </button>
          </div>
        </div>
      </motion.div>
    </div>
  );
};

export default Auth;