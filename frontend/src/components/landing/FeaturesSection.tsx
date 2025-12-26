import { motion, useInView, AnimatePresence } from 'framer-motion';
import { useRef, useState } from 'react';
import { 
  Brain, 
  Zap, 
  Shield, 
  RefreshCw, 
  Network, 
  Bell,
  Download,
  Search,
  Lightbulb
} from 'lucide-react';

const features = [
  {
    icon: Brain,
    title: 'Entity-Centric Memory',
    description: 'Organize knowledge around real-world entities with semantic relationships.',
  },
  {
    icon: RefreshCw,
    title: 'Continuous Reflection',
    description: 'Synthesize insights through asynchronous reflection loops.',
  },
  {
    icon: Network,
    title: 'Knowledge Graph',
    description: 'Interactive visualization of entities and their interconnections.',
  },
  {
    icon: Zap,
    title: 'Sub-100ms Latency',
    description: 'Blazing fast context retrieval with intelligent caching.',
  },
  {
    icon: Bell,
    title: 'Proactive Alerts',
    description: 'Automatic detection of conflicts and context-aware warnings.',
  },
  {
    icon: Shield,
    title: 'Enterprise Security',
    description: 'AES-256 encryption, HIPAA/SOC2 ready with granular controls.',
  },
];

const phases = [
  {
    id: 'ingestion',
    label: 'Ingestion',
    description: 'Extract entities and relationships from conversations in real-time',
    details: [
      'Named entity recognition from text',
      'Relationship extraction between entities',
      'Real-time streaming ingestion',
      'Automatic deduplication & merging',
    ],
    icon: Download,
  },
  {
    id: 'consultation',
    label: 'Consultation',
    description: 'Retrieve relevant context with sub-100ms latency',
    details: [
      'Semantic similarity search',
      'Graph traversal for related entities',
      'Intelligent context ranking',
      'Cached query optimization',
    ],
    icon: Search,
  },
  {
    id: 'reflection',
    label: 'Reflection',
    description: 'Synthesize insights and detect patterns asynchronously',
    details: [
      'Pattern detection across conversations',
      'Conflict identification & resolution',
      'Knowledge consolidation',
      'Proactive insight generation',
    ],
    icon: Lightbulb,
  },
];

// Staggered animation variants
const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.08,
      delayChildren: 0.1,
    },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 20 },
  visible: {
    opacity: 1,
    y: 0,
    transition: {
      duration: 0.5,
      ease: [0.25, 0.46, 0.45, 0.94],
    },
  },
};

// Large flowchart component with hover-to-expand
function MemoryLoopFlowchart() {
  const [hoveredPhase, setHoveredPhase] = useState<string | null>(null);

  const selectedPhase = phases.find(p => p.id === hoveredPhase);
  
  return (
    <div className="relative flex flex-col items-center w-full">
      {/* Flowchart */}
      <div className="flex items-center justify-center gap-4 md:gap-8 lg:gap-12 w-full">
        {phases.map((phase, index) => (
          <div key={phase.id} className="flex items-center gap-4 md:gap-8 lg:gap-12">
            {/* Node */}
            <div
              className="relative"
              onMouseEnter={() => setHoveredPhase(phase.id)}
              onMouseLeave={() => setHoveredPhase(null)}
            >
              <motion.div
                className={`
                  w-24 h-24 md:w-28 md:h-28 lg:w-32 lg:h-32 rounded-2xl border-2 cursor-pointer
                  flex flex-col items-center justify-center gap-2
                  transition-all duration-300
                  ${hoveredPhase === phase.id 
                    ? 'bg-primary/15 border-primary shadow-xl shadow-primary/25 ring-2 ring-primary/20' 
                    : 'bg-card/50 border-border/50 hover:border-primary/50'
                  }
                `}
                whileHover={{ scale: 1.06 }}
                whileTap={{ scale: 0.98 }}
              >
                <phase.icon className={`w-7 h-7 md:w-8 md:h-8 lg:w-9 lg:h-9 transition-colors duration-300 ${
                  hoveredPhase === phase.id ? 'text-primary' : 'text-muted-foreground'
                }`} />
                <span className={`text-xs md:text-sm lg:text-base font-medium transition-colors duration-300 ${
                  hoveredPhase === phase.id ? 'text-foreground' : 'text-muted-foreground'
                }`}>
                  {phase.label}
                </span>
              </motion.div>
            </div>

            {/* Arrow connector */}
            {index < phases.length - 1 && (
              <div className="flex items-center">
                <div className="w-10 md:w-16 lg:w-24 h-0.5 bg-border" />
                <div className="w-0 h-0 border-t-[5px] border-t-transparent border-b-[5px] border-b-transparent border-l-[8px] border-l-border" />
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Loop back arrow - enlarged */}
      <svg 
        className="w-full max-w-4xl mt-5 h-20 text-border"
        viewBox="0 0 500 80" 
        fill="none"
        preserveAspectRatio="xMidYMid meet"
      >
        <path
          d="M 460 10 C 460 65, 40 65, 40 10"
          stroke="currentColor"
          strokeWidth="2"
          fill="none"
          strokeDasharray="10 6"
        />
        <polygon
          points="35,10 47,4 47,16"
          fill="currentColor"
        />
      </svg>
      
      <span className="text-sm text-muted-foreground mt-1">Continuous loop</span>

      {/* Hover details panel */}
      <div className="h-48 mt-8 w-full max-w-2xl">
        <AnimatePresence mode="wait">
          {selectedPhase ? (
            <motion.div
              key={selectedPhase.id}
              initial={{ opacity: 0, y: 15 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -10 }}
              transition={{ duration: 0.25, ease: [0.25, 0.46, 0.45, 0.94] }}
              className="w-full"
            >
              <div className="bg-card/70 backdrop-blur-sm border border-border rounded-2xl p-6 md:p-8">
                <div className="flex items-start gap-5">
                  <div className="w-14 h-14 rounded-xl bg-primary/10 border border-primary/20 flex items-center justify-center shrink-0">
                    <selectedPhase.icon className="w-7 h-7 text-primary" />
                  </div>
                  <div className="flex-1">
                    <h4 className="text-xl font-semibold text-foreground mb-2">{selectedPhase.label}</h4>
                    <p className="text-muted-foreground mb-4">{selectedPhase.description}</p>
                    
                    <ul className="grid grid-cols-1 md:grid-cols-2 gap-2">
                      {selectedPhase.details.map((detail, i) => (
                        <motion.li
                          key={i}
                          initial={{ opacity: 0, x: -10 }}
                          animate={{ opacity: 1, x: 0 }}
                          transition={{ delay: i * 0.05 }}
                          className="flex items-center gap-2 text-sm text-foreground/80"
                        >
                          <div className="w-1.5 h-1.5 rounded-full bg-primary shrink-0" />
                          {detail}
                        </motion.li>
                      ))}
                    </ul>
                  </div>
                </div>
              </div>
            </motion.div>
          ) : (
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="flex items-center justify-center h-full"
            >
              <p className="text-muted-foreground text-center">
                Hover over a phase to see details
              </p>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
}

export function FeaturesSection() {
  const ref = useRef(null);
  const isInView = useInView(ref, { once: true, margin: '-100px' });

  return (
    <section ref={ref} id="features" className="py-24 md:py-32 relative">
      {/* Subtle background */}
      <div className="absolute inset-0 bg-gradient-to-b from-transparent via-muted/30 to-transparent" />

      <div className="container mx-auto px-4 relative z-10">
        {/* Section header */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={isInView ? { opacity: 1, y: 0 } : {}}
          transition={{ duration: 0.5 }}
          className="text-center mb-16"
        >
          <p className="text-primary text-sm font-medium mb-3">Features</p>
          <h2 className="text-3xl md:text-4xl font-semibold mb-4 tracking-tight">
            Memory that evolves with your AI
          </h2>
          <p className="text-muted-foreground max-w-xl mx-auto">
            A revolutionary approach to AI context management that goes beyond simple retrieval.
          </p>
        </motion.div>

        {/* Features grid with staggered animations */}
        <motion.div 
          className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mb-24"
          variants={containerVariants}
          initial="hidden"
          animate={isInView ? "visible" : "hidden"}
        >
          {features.map((feature) => (
            <motion.div
              key={feature.title}
              variants={itemVariants}
              className="group"
            >
              <div className="p-6 rounded-lg border border-border/50 bg-card/30 hover:bg-card/60 hover:border-border transition-all duration-300 h-full">
                <div className="w-9 h-9 rounded-md bg-primary/10 flex items-center justify-center mb-4 group-hover:bg-primary/20 transition-colors">
                  <feature.icon className="w-4 h-4 text-primary" />
                </div>
                <h3 className="text-base font-medium mb-2">{feature.title}</h3>
                <p className="text-sm text-muted-foreground leading-relaxed">{feature.description}</p>
              </div>
            </motion.div>
          ))}
        </motion.div>

        {/* Memory Loop Flowchart - Full width */}
        <motion.div
          initial={{ opacity: 0, y: 30 }}
          animate={isInView ? { opacity: 1, y: 0 } : {}}
          transition={{ duration: 0.6, delay: 0.4 }}
          className="w-full"
        >
          <div className="text-center mb-10">
            <h3 className="text-2xl font-semibold mb-3 tracking-tight">The Memory Loop</h3>
            <p className="text-muted-foreground text-sm">
              Hover over each phase to learn more.
            </p>
          </div>

          <MemoryLoopFlowchart />
        </motion.div>
      </div>
    </section>
  );
}
