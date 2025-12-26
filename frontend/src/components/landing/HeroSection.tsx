import { motion, AnimatePresence } from 'framer-motion';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { AbstractArt } from '@/components/auth/AbstractArt';
import { ArrowRight, Loader2, Eye, EyeOff, ArrowLeft, Shield } from 'lucide-react';
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';
import { useToast } from '@/hooks/use-toast';
import { z } from 'zod';

// Validation schemas
const usernameSchema = z.string().min(3, { message: "Username must be at least 3 characters" });
const passwordSchema = z.string().min(6, { message: "Password must be at least 6 characters" });

export function HeroSection() {
  const [isTransitioning, setIsTransitioning] = useState(false);
  const [isReversing, setIsReversing] = useState(false);
  const [showLoginForm, setShowLoginForm] = useState(false);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [role, setRole] = useState<'user' | 'admin'>('user');
  const [isLoading, setIsLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [isSignUp, setIsSignUp] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const navigate = useNavigate();
  const { toast } = useToast();
  const { login, register } = useAuth();

  const handleGetStarted = () => {
    setIsTransitioning(true);
    setIsReversing(false);
    setShowLoginForm(true);
  };

  const handleBackToHome = () => {
    setIsReversing(true);
    setTimeout(() => {
      setShowLoginForm(false);
      setIsTransitioning(false);
      setIsReversing(false);
    }, 650);
  };

  const validateForm = () => {
    const formErrors: Record<string, string> = {};

    const usernameResult = usernameSchema.safeParse(username);
    if (!usernameResult.success) {
      formErrors.username = usernameResult.error.errors[0].message;
    }

    const passwordResult = passwordSchema.safeParse(password);
    if (!passwordResult.success && isSignUp) {
      formErrors.password = passwordResult.error.errors[0].message;
    } else if (!password && !isSignUp) {
      formErrors.password = "Password is required";
    }

    if (isSignUp && password !== confirmPassword) {
      formErrors.confirmPassword = "Passwords don't match";
    }

    setErrors(formErrors);
    return Object.keys(formErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateForm()) return;

    setIsLoading(true);

    try {
      if (isSignUp) {
        const result = await register(username, password, role);
        if (!result.success) {
          toast({ variant: "destructive", title: "Sign up failed", description: result.error || "Failed to create account" });
        } else {
          toast({ title: "Account created!", description: `Welcome ${username}! You are now logged in as ${role}.` });
          navigate('/dashboard');
        }
      } else {
        const result = await login(username, password);
        if (!result.success) {
          toast({ variant: "destructive", title: "Login failed", description: result.error || "Invalid credentials" });
        } else {
          toast({ title: "Welcome back!", description: `Logged in as ${username}` });
          navigate('/dashboard');
        }
      }
    } catch (error) {
      toast({ variant: "destructive", title: "Error", description: "An unexpected error occurred." });
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
    <section className="relative min-h-screen flex items-center justify-center overflow-hidden pt-20">

      {/* Gradient overlays - fade out when transitioning */}
      <AnimatePresence>
        {!isTransitioning && (
          <motion.div
            initial={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.6 }}
            className="absolute inset-0 z-10 pointer-events-none"
          >
            <div className="absolute inset-0 bg-gradient-to-b from-background/90 via-background/50 to-background/80" />
            <div className="absolute inset-0 bg-gradient-radial from-primary/5 via-transparent to-transparent" />
          </motion.div>
        )}
      </AnimatePresence>

      {/* Gradient orbs - fade out when transitioning */}
      <AnimatePresence>
        {!isTransitioning && (
          <motion.div
            initial={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.5 }}
          >
            <div className="absolute top-1/4 left-1/4 w-[500px] h-[500px] rounded-full bg-primary/5 blur-[120px] pointer-events-none" />
            <div className="absolute bottom-1/4 right-1/4 w-[400px] h-[400px] rounded-full bg-primary/3 blur-[100px] pointer-events-none" />
          </motion.div>
        )}
      </AnimatePresence>

      {/* Full-screen Border Animation with Complete Auth Page Inside */}
      <AnimatePresence>
        {isTransitioning && (
          <motion.div
            className="fixed inset-0 z-[100] flex items-center justify-center"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.1, ease: 'linear' }}
          >
            {/* Background that fades in/out */}
            <motion.div
              className="absolute inset-0 bg-background"
              initial={{ opacity: 0 }}
              animate={{ opacity: isReversing ? 0 : 1 }}
              transition={{
                duration: isReversing ? 0.35 : 0.4,
                delay: isReversing ? 0 : 0.15,
                ease: [0.32, 0.72, 0, 1]
              }}
            />

            {/* Animated glowing border rectangle */}
            <motion.div
              className="absolute flex overflow-hidden will-change-transform"
              initial={{
                width: '100px',
                height: '100px',
                borderRadius: '24px',
                scale: 0.9,
              }}
              animate={{
                width: isReversing ? '100px' : '100vw',
                height: isReversing ? '100px' : '100vh',
                borderRadius: isReversing ? '24px' : '0px',
                scale: isReversing ? 0.9 : 1,
              }}
              style={{
                background: 'hsl(var(--background))',
                border: '2px solid hsl(var(--primary) / 0.4)',
                boxShadow: `
                  0 0 60px 20px hsl(var(--primary) / 0.2),
                  0 0 120px 40px hsl(var(--primary) / 0.1),
                  inset 0 0 30px 10px hsl(var(--primary) / 0.05)
                `,
              }}
              transition={{
                type: "spring",
                stiffness: 100,
                damping: 18,
                mass: 0.8,
              }}
            >
              {/* Auth Page Content */}
              <AnimatePresence mode="wait">
                {showLoginForm && (
                  <motion.div
                    className="flex"
                    style={{
                      width: '100vw',
                      height: '100vh',
                      minWidth: '100vw',
                      minHeight: '100vh',
                    }}
                    initial={{ filter: 'blur(6px)', opacity: 0.7 }}
                    animate={{
                      filter: isReversing ? 'blur(6px)' : 'blur(0px)',
                      opacity: isReversing ? 0.7 : 1,
                    }}
                    transition={{
                      duration: 0.4,
                      delay: isReversing ? 0.1 : 0.2,
                      ease: [0.22, 1, 0.36, 1]
                    }}
                  >
                    {/* Left side - Abstract Art (hidden on mobile) */}
                    <div className="hidden lg:block lg:w-1/2 xl:w-[55%] relative">
                      <AbstractArt />
                    </div>

                    {/* Right side - Auth form */}
                    <div className="w-full lg:w-1/2 xl:w-[45%] flex items-center justify-center p-6 lg:p-12 overflow-y-auto">
                      <motion.div
                        className="w-full max-w-md"
                        initial={{ scale: 0.92, opacity: 0 }}
                        animate={{
                          scale: isReversing ? 0.92 : 1,
                          opacity: isReversing ? 0 : 1
                        }}
                        transition={{
                          type: "spring",
                          stiffness: 300,
                          damping: 24,
                          mass: 0.8,
                          delay: isReversing ? 0 : 0.3,
                        }}
                      >
                        {/* Back button */}
                        <button
                          onClick={handleBackToHome}
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
                      </motion.div>
                    </div>
                  </motion.div>
                )}
              </AnimatePresence>
            </motion.div>

            {/* Completion glow pulse */}
            <motion.div
              className="fixed inset-0 pointer-events-none"
              initial={{ opacity: 0 }}
              animate={{ opacity: [0, 0.15, 0] }}
              transition={{ duration: 0.6, delay: isReversing ? 0.25 : 0.5, ease: [0.32, 0.72, 0, 1] }}
              key={isReversing ? 'reverse-pulse' : 'forward-pulse'}
              style={{ background: 'radial-gradient(ellipse at center, hsl(var(--primary) / 0.3) 0%, transparent 70%)' }}
            />

            {/* Outer glowing ring effect */}
            <motion.div
              className="absolute pointer-events-none will-change-transform"
              initial={{ width: '120px', height: '120px', borderRadius: '24px', opacity: 0.8, scale: 0.9 }}
              animate={{
                width: isReversing ? '120px' : '105vw',
                height: isReversing ? '120px' : '105vh',
                borderRadius: isReversing ? '24px' : '0px',
                opacity: isReversing ? 0.8 : 0,
                scale: isReversing ? 0.9 : 1,
              }}
              style={{ border: '2px solid hsl(var(--primary) / 0.3)', boxShadow: '0 0 60px 20px hsl(var(--primary) / 0.2)' }}
              transition={{ type: "spring", stiffness: 100, damping: 18, mass: 0.8 }}
            />

            {/* Core bright glow */}
            <motion.div
              className="absolute rounded-full pointer-events-none will-change-transform"
              style={{ background: 'radial-gradient(circle, hsl(var(--primary) / 0.7) 0%, hsl(var(--primary) / 0.3) 50%, transparent 80%)' }}
              initial={{ width: '40px', height: '40px', opacity: 1, scale: 0.8 }}
              animate={{
                width: isReversing ? '40px' : '150px',
                height: isReversing ? '40px' : '150px',
                opacity: isReversing ? 1 : 0,
                scale: isReversing ? 0.8 : 1.2,
              }}
              transition={{ type: "spring", stiffness: 120, damping: 16, mass: 0.6 }}
            />
          </motion.div>
        )}
      </AnimatePresence>

      {/* Content - Landing page content */}
      <AnimatePresence>
        {!isTransitioning && (
          <motion.div className="relative z-20 container mx-auto px-4 text-center max-w-4xl">
            {/* Badge */}
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -30, scale: 1.1, filter: 'blur(4px)' }}
              transition={{ duration: 0.5, delay: 0.1, exit: { duration: 0.3, delay: 0.2 } }}
              className="mb-8"
            >
              <span className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-primary/10 border border-primary/20 text-xs font-medium text-primary">
                <span className="w-1.5 h-1.5 rounded-full bg-primary animate-pulse" />
                Now in Beta
              </span>
            </motion.div>

            {/* Heading */}
            <motion.h1
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -25, scale: 1.05, filter: 'blur(6px)' }}
              transition={{ duration: 0.5, delay: 0.2, exit: { duration: 0.35, delay: 0.15 } }}
              className="text-4xl sm:text-5xl md:text-6xl lg:text-7xl font-semibold mb-6 leading-[1.1] tracking-tight"
            >
              <span className="text-foreground">AI memory that</span>
              <br />
              <span className="text-gradient">thinks like you do</span>
            </motion.h1>

            {/* Description */}
            <motion.p
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -20, scale: 1.03, filter: 'blur(5px)' }}
              transition={{ duration: 0.5, delay: 0.3, exit: { duration: 0.3, delay: 0.1 } }}
              className="text-lg md:text-xl text-muted-foreground max-w-2xl mx-auto mb-10 leading-relaxed"
            >
              The Reflective Memory Kernel is a persistent, entity-centric memory system
              that evolves through continuous reflection. Built for production AI.
            </motion.p>

            {/* Buttons */}
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -15, scale: 1.08, filter: 'blur(6px)' }}
              transition={{ duration: 0.5, delay: 0.4, exit: { duration: 0.25, delay: 0.05 } }}
              className="flex flex-col sm:flex-row gap-3 justify-center items-center relative z-50"
            >
              <Button
                size="lg"
                className="group h-11 px-6 text-sm font-medium relative z-50 cursor-pointer"
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  handleGetStarted();
                }}
                type="button"
              >
                Get started
                <ArrowRight className="w-4 h-4 ml-2 group-hover:translate-x-0.5 transition-transform" />
              </Button>
              <Button variant="ghost" size="lg" className="h-11 px-6 text-sm font-medium text-muted-foreground hover:text-foreground">
                Read documentation
              </Button>
            </motion.div>

            {/* Stats - Glass Card Container */}
            <motion.div
              initial={{ opacity: 0, y: 30 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: 10, filter: 'blur(4px)' }}
              transition={{ duration: 0.6, delay: 0.5, exit: { duration: 0.2, delay: 0 } }}
              className="mt-16 relative"
            >
              <div className="relative mx-auto max-w-4xl">
                <div className="absolute inset-0 bg-gradient-to-b from-primary/10 via-primary/5 to-transparent rounded-[2rem] blur-2xl" />
                <div
                  className="relative backdrop-blur-xl rounded-[2rem] border border-white/10 overflow-hidden"
                  style={{
                    background: 'linear-gradient(180deg, hsl(var(--background) / 0.7) 0%, hsl(var(--background) / 0.4) 100%)',
                    boxShadow: '0 0 0 1px hsl(var(--primary) / 0.1), 0 8px 40px -12px hsl(var(--primary) / 0.15), inset 0 1px 0 0 hsl(0 0% 100% / 0.05)',
                  }}
                >
                  <div className="absolute inset-0 bg-gradient-to-b from-white/5 via-transparent to-transparent pointer-events-none" />
                  <div className="relative px-8 py-10">
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-8">
                      {[
                        { value: '10M+', label: 'Entities processed' },
                        { value: '<100ms', label: 'Avg latency' },
                        { value: '99.9%', label: 'Uptime SLA' },
                        { value: '500+', label: 'Teams using' },
                      ].map((stat, i) => (
                        <motion.div
                          key={stat.label}
                          initial={{ opacity: 0, y: 10 }}
                          animate={{ opacity: 1, y: 0 }}
                          transition={{ duration: 0.4, delay: 0.65 + i * 0.08 }}
                          className="text-center"
                        >
                          <div className="text-2xl md:text-3xl font-semibold text-foreground mb-1">{stat.value}</div>
                          <div className="text-xs text-muted-foreground">{stat.label}</div>
                        </motion.div>
                      ))}
                    </div>
                  </div>
                  <div
                    className="absolute bottom-0 left-1/2 -translate-x-1/2 w-3/4 h-px"
                    style={{ background: 'linear-gradient(90deg, transparent 0%, hsl(var(--primary) / 0.3) 50%, transparent 100%)' }}
                  />
                </div>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </section>
  );
}