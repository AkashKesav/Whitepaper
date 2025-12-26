import { motion, useScroll, useTransform, useSpring, useMotionValue, MotionValue } from 'framer-motion';
import { useRef, useEffect, useState } from 'react';

const dashboardCards = [
  {
    title: 'Memory Graph',
    rows: [
      { label: 'Entities', value: '12,847', trend: '+12%' },
      { label: 'Connections', value: '45,291', trend: '+8%' },
      { label: 'Patterns', value: '1,203', trend: '+23%' },
    ],
  },
  {
    title: 'Active Sessions',
    rows: [
      { label: 'Real-time', value: '847', trend: 'live' },
      { label: 'Queries/sec', value: '2.4k', trend: '+5%' },
      { label: 'Latency', value: '42ms', trend: '-15%' },
    ],
  },
  {
    title: 'Reflection Loop',
    rows: [
      { label: 'Insights', value: '324', trend: '+18%' },
      { label: 'Conflicts', value: '12', trend: '-40%' },
      { label: 'Suggestions', value: '89', trend: '+7%' },
    ],
  },
];

function DashboardCard({ 
  card, 
  index, 
  scrollProgress,
  totalCards,
  floatOffset,
  mouseX,
  mouseY,
  isZooming
}: { 
  card: typeof dashboardCards[0]; 
  index: number; 
  scrollProgress: any;
  totalCards: number;
  floatOffset: number;
  mouseX: any;
  mouseY: any;
  isZooming: boolean;
}) {
  const reverseIndex = totalCards - 1 - index;
  const baseOffset = reverseIndex * 70;
  const baseRotation = reverseIndex * 3;
  const baseScale = 1 - reverseIndex * 0.05;
  
  // Parallax depth multiplier - back cards move faster for depth effect
  const depthMultiplier = 1 + reverseIndex * 0.6;
  
  // Mouse parallax - different intensity per card layer
  const parallaxIntensity = (reverseIndex + 1) * 8;
  const mouseXSpring = useSpring(mouseX, { stiffness: 100, damping: 30 });
  const mouseYSpring = useSpring(mouseY, { stiffness: 100, damping: 30 });
  
  const rotateY = useTransform(mouseXSpring, [-1, 1], [-parallaxIntensity, parallaxIntensity]);
  const rotateXMouse = useTransform(mouseYSpring, [-1, 1], [parallaxIntensity, -parallaxIntensity]);
  const translateX = useTransform(mouseXSpring, [-1, 1], [-parallaxIntensity * 2, parallaxIntensity * 2]);
  const translateYMouse = useTransform(mouseYSpring, [-1, 1], [-parallaxIntensity * 1.5, parallaxIntensity * 1.5]);
  
  // Scroll-based transforms with parallax depth - back cards scroll faster
  const scrollY = useTransform(
    scrollProgress,
    [0, 0.25, 0.5, 0.75, 1],
    [
      baseOffset, 
      baseOffset + 15 * depthMultiplier, 
      baseOffset - 80 * depthMultiplier, 
      -200 * depthMultiplier - reverseIndex * 80,
      -400 * depthMultiplier - reverseIndex * 120
    ]
  );
  
  // Z-axis parallax - cards spread apart in depth as you scroll
  const scrollZ = useTransform(
    scrollProgress,
    [0, 0.3, 0.6, 1],
    [0, -30 * reverseIndex, -80 * reverseIndex, -150 * reverseIndex]
  );
  
  const scrollRotateX = useTransform(
    scrollProgress,
    [0, 0.25, 0.5, 0.75, 1],
    [
      baseRotation, 
      baseRotation + 4 * depthMultiplier, 
      baseRotation + 10 * depthMultiplier, 
      15 + reverseIndex * 5,
      25 + reverseIndex * 8
    ]
  );
  
  // Subtle X-axis spread as cards scroll - creates fanning effect
  const scrollSpreadX = useTransform(
    scrollProgress,
    [0, 0.4, 0.8, 1],
    [0, reverseIndex * 15, reverseIndex * 40, reverseIndex * 60]
  );
  
  const opacity = useTransform(
    scrollProgress,
    [0, 0.2, 0.5, 0.8, 1],
    [1 - reverseIndex * 0.05, 1 - reverseIndex * 0.03, 0.9 - reverseIndex * 0.1, 0.5 - reverseIndex * 0.15, 0.1]
  );
  
  const scale = useTransform(
    scrollProgress,
    [0, 0.3, 0.6, 1],
    [baseScale, baseScale + 0.02, baseScale - 0.05, baseScale - 0.15]
  );
  
  // Dynamic shadow that grows as cards spread apart
  const shadowBlur = useTransform(
    scrollProgress,
    [0, 0.3, 0.6, 1],
    [20, 30 + reverseIndex * 10, 50 + reverseIndex * 15, 80 + reverseIndex * 20]
  );
  
  const shadowY = useTransform(
    scrollProgress,
    [0, 0.3, 0.6, 1],
    [10, 15 + reverseIndex * 5, 25 + reverseIndex * 8, 40 + reverseIndex * 12]
  );
  
  const shadowOpacity = useTransform(
    scrollProgress,
    [0, 0.3, 0.6, 1],
    [0.4, 0.5 + reverseIndex * 0.05, 0.6 + reverseIndex * 0.08, 0.7]
  );

  // Smooth spring for float animation
  const springConfig = { stiffness: 100, damping: 30 };
  const floatY = useSpring(floatOffset * (reverseIndex + 1) * 0.5, springConfig);

  return (
    <motion.div
      style={{
        y: scrollY,
        z: scrollZ,
        x: scrollSpreadX,
        rotateX: scrollRotateX,
        rotateY: rotateY,
        opacity,
        scale,
        translateX: translateX,
        translateY: floatY,
        transformStyle: 'preserve-3d',
        transformOrigin: 'center center',
        boxShadow: useTransform(
          [shadowBlur, shadowY, shadowOpacity],
          ([blur, y, op]) => `0 ${y}px ${blur}px -10px rgba(0, 0, 0, ${op})`
        ),
      }}
      animate={isZooming ? {
        z: index === 0 ? 800 : -200 - reverseIndex * 200,
        opacity: index === 0 ? 1 : 0,
        scale: index === 0 ? 1.5 : 0.5,
      } : {}}
      transition={{ duration: 0.8, ease: [0.4, 0, 0.2, 1] }}
      className="absolute inset-0 rounded-xl"
    >
      <div className="w-full h-full rounded-xl border border-border/70 bg-card/95 backdrop-blur-md overflow-hidden">
        {/* Header */}
        <div className="px-5 py-3.5 border-b border-border/50 flex items-center justify-between bg-muted/50">
          <div className="flex items-center gap-3">
            <div className="flex gap-1.5">
              <div className="w-3 h-3 rounded-full bg-red-500/90" />
              <div className="w-3 h-3 rounded-full bg-yellow-500/90" />
              <div className="w-3 h-3 rounded-full bg-green-500/90" />
            </div>
            <span className="text-sm text-foreground font-medium ml-2">{card.title}</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-5 h-5 rounded bg-muted/70" />
            <div className="w-5 h-5 rounded bg-muted/70" />
            <div className="w-5 h-5 rounded bg-muted/70" />
          </div>
        </div>
        
        {/* Sidebar + Content */}
        <div className="flex h-[calc(100%-52px)]">
          {/* Sidebar */}
          <div className="w-14 border-r border-border/40 p-2.5 flex flex-col gap-2 bg-muted/40">
            {[...Array(6)].map((_, i) => (
              <div 
                key={i} 
                className={`w-full h-7 rounded-md ${i === 0 ? 'bg-primary/40 border border-primary/50' : 'bg-muted/50'}`} 
              />
            ))}
          </div>
          
          {/* Main content */}
          <div className="flex-1 p-5 bg-background/70">
            {/* Stats row */}
            <div className="grid grid-cols-3 gap-4 mb-5">
              {card.rows.map((row, i) => (
                <div key={i} className="p-3 rounded-lg bg-card border border-border/50">
                  <div className="text-xs text-muted-foreground mb-1.5">{row.label}</div>
                  <div className="flex items-baseline gap-2">
                    <span className="text-lg font-semibold text-foreground">{row.value}</span>
                    <span className={`text-xs font-medium ${row.trend.startsWith('+') ? 'text-green-400' : row.trend.startsWith('-') ? 'text-red-400' : 'text-primary'}`}>
                      {row.trend}
                    </span>
                  </div>
                </div>
              ))}
            </div>
            
            {/* Chart */}
            <div className="h-24 rounded-lg bg-gradient-to-b from-primary/10 to-primary/20 border border-border/40 flex items-end justify-around px-3 pb-3">
              {[35, 55, 40, 70, 50, 65, 55, 80, 45, 75, 60, 85, 50, 70].map((h, i) => (
                <div 
                  key={i} 
                  className="w-2 bg-gradient-to-t from-primary/90 to-primary/50 rounded-t"
                  style={{ height: `${h}%` }}
                />
              ))}
            </div>
            
            {/* Table */}
            <div className="mt-5 space-y-2">
              {[...Array(3)].map((_, i) => (
                <div key={i} className="flex items-center gap-3 p-2.5 rounded-lg bg-muted/40 border border-border/30">
                  <div className="w-8 h-8 rounded-md bg-primary/30" />
                  <div className="flex-1">
                    <div className="h-2.5 w-28 bg-foreground/30 rounded mb-1" />
                    <div className="h-2 w-20 bg-muted-foreground/30 rounded" />
                  </div>
                  <div className="h-2.5 w-14 bg-muted/60 rounded" />
                  <div className="h-6 w-16 bg-primary/30 rounded-md" />
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </motion.div>
  );
}

interface StackedDashboardsProps {
  isZooming?: boolean;
  scrollProgress?: MotionValue<number>;
}

export function StackedDashboards({ isZooming = false, scrollProgress }: StackedDashboardsProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [floatOffset, setFloatOffset] = useState(0);
  
  // Mouse tracking
  const mouseX = useMotionValue(0);
  const mouseY = useMotionValue(0);
  
  // Internal scroll progress if not provided externally
  const { scrollYProgress: internalProgress } = useScroll({
    target: containerRef,
    offset: ['start start', 'end start'],
  });
  
  const effectiveProgress = scrollProgress || internalProgress;
  
  // Apple-style fade out - smooth exit as user scrolls past features
  const containerOpacity = useTransform(
    effectiveProgress,
    [0, 0.4, 0.7, 1],
    [1, 1, 0.3, 0]
  );
  
  // Smooth scale down as it fades
  const containerScale = useTransform(
    effectiveProgress,
    [0, 0.5, 1],
    [1, 0.95, 0.85]
  );
  
  // Blur effect for Apple-style transition
  const containerBlur = useTransform(
    effectiveProgress,
    [0, 0.5, 1],
    [0, 2, 8]
  );

  // Mouse move handler
  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      const { clientX, clientY } = e;
      const { innerWidth, innerHeight } = window;
      
      const x = (clientX / innerWidth) * 2 - 1;
      const y = (clientY / innerHeight) * 2 - 1;
      
      mouseX.set(x);
      mouseY.set(y);
    };
    
    window.addEventListener('mousemove', handleMouseMove);
    return () => window.removeEventListener('mousemove', handleMouseMove);
  }, [mouseX, mouseY]);

  // Floating animation
  useEffect(() => {
    let animationFrame: number;
    let startTime = Date.now();
    
    const animate = () => {
      const elapsed = (Date.now() - startTime) / 1000;
      const newOffset = Math.sin(elapsed * 0.8) * 8;
      setFloatOffset(newOffset);
      animationFrame = requestAnimationFrame(animate);
    };
    
    animationFrame = requestAnimationFrame(animate);
    
    return () => {
      cancelAnimationFrame(animationFrame);
    };
  }, []);

  return (
    <motion.div 
      ref={containerRef} 
      className="fixed inset-0 flex items-center justify-center overflow-hidden pointer-events-none"
      style={{ 
        opacity: containerOpacity,
        scale: containerScale,
        filter: useTransform(containerBlur, (v) => `blur(${v}px)`),
      }}
    >
      {/* Subtle gradient fades */}
      <div className="absolute inset-x-0 top-0 h-24 bg-gradient-to-b from-background/80 to-transparent z-20" />
      <div className="absolute inset-x-0 bottom-0 h-24 bg-gradient-to-t from-background/80 to-transparent z-20" />
      <div className="absolute inset-y-0 left-0 w-16 bg-gradient-to-r from-background/60 to-transparent z-20" />
      <div className="absolute inset-y-0 right-0 w-16 bg-gradient-to-l from-background/60 to-transparent z-20" />
      
      {/* Stacked cards container */}
      <motion.div 
        className="relative w-[900px] h-[560px]"
        style={{ 
          perspective: '2000px',
          perspectiveOrigin: 'center 40%',
        }}
        initial={{ opacity: 0, y: 40 }}
        animate={{ 
          opacity: 1, 
          y: 0,
          scale: isZooming ? 1.1 : 1,
        }}
        transition={{ duration: 0.8, delay: 0.3 }}
      >
        {dashboardCards.map((card, index) => (
          <DashboardCard
            key={index}
            card={card}
            index={index}
            scrollProgress={effectiveProgress}
            totalCards={dashboardCards.length}
            floatOffset={floatOffset}
            mouseX={mouseX}
            mouseY={mouseY}
            isZooming={isZooming}
          />
        ))}
      </motion.div>
    </motion.div>
  );
}