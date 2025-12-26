import { motion } from 'framer-motion';

export function AbstractArt() {
  return (
    <div className="relative w-full h-full overflow-hidden bg-gradient-to-br from-background via-muted/30 to-background">
      {/* Animated gradient orbs */}
      <motion.div
        className="absolute top-1/4 left-1/4 w-[600px] h-[600px] rounded-full"
        style={{
          background: 'radial-gradient(circle, hsl(var(--primary) / 0.3) 0%, transparent 70%)',
        }}
        animate={{
          x: [0, 50, -30, 0],
          y: [0, -40, 60, 0],
          scale: [1, 1.1, 0.95, 1],
        }}
        transition={{
          duration: 20,
          repeat: Infinity,
          ease: 'easeInOut',
        }}
      />
      
      <motion.div
        className="absolute bottom-1/4 right-1/4 w-[500px] h-[500px] rounded-full"
        style={{
          background: 'radial-gradient(circle, hsl(var(--primary) / 0.2) 0%, transparent 70%)',
        }}
        animate={{
          x: [0, -60, 40, 0],
          y: [0, 50, -30, 0],
          scale: [1, 0.9, 1.15, 1],
        }}
        transition={{
          duration: 25,
          repeat: Infinity,
          ease: 'easeInOut',
        }}
      />

      <motion.div
        className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[400px] h-[400px] rounded-full"
        style={{
          background: 'radial-gradient(circle, hsl(var(--primary) / 0.15) 0%, transparent 70%)',
        }}
        animate={{
          scale: [1, 1.2, 1],
          opacity: [0.5, 0.8, 0.5],
        }}
        transition={{
          duration: 15,
          repeat: Infinity,
          ease: 'easeInOut',
        }}
      />

      {/* Grid pattern overlay */}
      <div 
        className="absolute inset-0 opacity-[0.03]"
        style={{
          backgroundImage: `
            linear-gradient(hsl(var(--foreground)) 1px, transparent 1px),
            linear-gradient(90deg, hsl(var(--foreground)) 1px, transparent 1px)
          `,
          backgroundSize: '60px 60px',
        }}
      />

      {/* Floating geometric shapes */}
      <motion.div
        className="absolute top-[20%] left-[15%] w-20 h-20 border border-primary/20 rounded-2xl"
        animate={{
          rotate: [0, 90, 180, 270, 360],
          y: [0, -20, 0, 20, 0],
        }}
        transition={{
          duration: 30,
          repeat: Infinity,
          ease: 'linear',
        }}
      />
      
      <motion.div
        className="absolute top-[60%] left-[25%] w-12 h-12 bg-primary/10 rounded-full"
        animate={{
          scale: [1, 1.3, 1],
          opacity: [0.3, 0.6, 0.3],
        }}
        transition={{
          duration: 8,
          repeat: Infinity,
          ease: 'easeInOut',
        }}
      />

      <motion.div
        className="absolute top-[35%] right-[20%] w-16 h-16 border border-primary/15 rotate-45"
        animate={{
          rotate: [45, 135, 225, 315, 405],
          scale: [1, 0.8, 1.1, 0.9, 1],
        }}
        transition={{
          duration: 25,
          repeat: Infinity,
          ease: 'easeInOut',
        }}
      />

      <motion.div
        className="absolute bottom-[25%] left-[40%] w-8 h-8 bg-primary/15 rounded-lg"
        animate={{
          y: [0, -30, 0],
          x: [0, 15, 0],
          rotate: [0, 45, 0],
        }}
        transition={{
          duration: 12,
          repeat: Infinity,
          ease: 'easeInOut',
        }}
      />

      {/* Animated lines */}
      <svg className="absolute inset-0 w-full h-full" xmlns="http://www.w3.org/2000/svg">
        <motion.line
          x1="10%"
          y1="30%"
          x2="40%"
          y2="70%"
          stroke="hsl(var(--primary) / 0.1)"
          strokeWidth="1"
          initial={{ pathLength: 0 }}
          animate={{ pathLength: 1 }}
          transition={{ duration: 3, repeat: Infinity, repeatType: 'reverse', ease: 'easeInOut' }}
        />
        <motion.line
          x1="60%"
          y1="20%"
          x2="30%"
          y2="80%"
          stroke="hsl(var(--primary) / 0.08)"
          strokeWidth="1"
          initial={{ pathLength: 0 }}
          animate={{ pathLength: 1 }}
          transition={{ duration: 4, delay: 1, repeat: Infinity, repeatType: 'reverse', ease: 'easeInOut' }}
        />
        <motion.circle
          cx="50%"
          cy="50%"
          r="100"
          fill="none"
          stroke="hsl(var(--primary) / 0.1)"
          strokeWidth="1"
          initial={{ scale: 0.5, opacity: 0 }}
          animate={{ scale: 1.5, opacity: [0, 0.5, 0] }}
          transition={{ duration: 6, repeat: Infinity, ease: 'easeOut' }}
        />
      </svg>

      {/* Content overlay */}
      <div className="absolute inset-0 flex flex-col justify-center items-start p-12 lg:p-16">
        <motion.div
          initial={{ opacity: 0, x: -30 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ duration: 0.8, delay: 0.3 }}
        >
          <div className="flex items-center gap-3 mb-6">
            <div className="w-10 h-10 bg-primary rounded-xl flex items-center justify-center">
              <div className="w-5 h-5 bg-primary-foreground rounded-md" />
            </div>
            <span className="text-xl font-semibold text-foreground">MemoryKernel</span>
          </div>
          
          <h2 className="text-3xl lg:text-4xl font-semibold text-foreground mb-4 leading-tight">
            AI memory that
            <br />
            <span className="text-gradient">thinks like you do</span>
          </h2>
          
          <p className="text-muted-foreground max-w-md leading-relaxed">
            The Reflective Memory Kernel is a persistent, entity-centric memory system 
            that evolves through continuous reflection.
          </p>
        </motion.div>

        {/* Stats at bottom */}
        <motion.div
          className="absolute bottom-12 lg:bottom-16 left-12 lg:left-16"
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, delay: 0.5 }}
        >
          <div className="flex gap-8">
            {[
              { value: '10M+', label: 'Entities' },
              { value: '<100ms', label: 'Latency' },
              { value: '99.9%', label: 'Uptime' },
            ].map((stat) => (
              <div key={stat.label}>
                <div className="text-2xl font-semibold text-foreground">{stat.value}</div>
                <div className="text-xs text-muted-foreground">{stat.label}</div>
              </div>
            ))}
          </div>
        </motion.div>
      </div>
    </div>
  );
}