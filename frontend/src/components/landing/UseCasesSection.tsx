import { motion, useInView } from 'framer-motion';
import { useRef, useState } from 'react';
import { DataFlow3D } from '@/components/3d/DataFlow3D';
import { 
  Stethoscope, 
  TrendingUp, 
  Users2, 
  Server,
} from 'lucide-react';

const useCases = [
  {
    id: 'healthcare',
    icon: Stethoscope,
    title: 'Healthcare',
    subtitle: 'Patient-Centric Care',
    description: 'Track patient histories, medication interactions, and care preferences across thousands of encounters.',
    stats: ['85% fewer missed interactions', '3x faster retrieval'],
  },
  {
    id: 'finance',
    icon: TrendingUp,
    title: 'Financial Services',
    subtitle: 'Intelligent Advisory',
    description: 'Remember client preferences, risk tolerances, and life events. Surface relevant opportunities.',
    stats: ['60% improved personalization', 'Real-time compliance'],
  },
  {
    id: 'sales',
    icon: Users2,
    title: 'Enterprise Sales',
    subtitle: 'Relationship Intelligence',
    description: 'Track deal histories, stakeholder preferences, and competitive dynamics.',
    stats: ['40% faster deal cycles', '90% context retention'],
  },
  {
    id: 'it',
    icon: Server,
    title: 'IT Operations',
    subtitle: 'Incident Intelligence',
    description: 'Connect recurring issues to root causes. Build institutional knowledge.',
    stats: ['70% faster resolution', 'Pattern prevention'],
  },
];

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.1,
      delayChildren: 0.2,
    },
  },
};

const itemVariants = {
  hidden: { opacity: 0, x: -20 },
  visible: {
    opacity: 1,
    x: 0,
    transition: {
      duration: 0.5,
      ease: [0.25, 0.46, 0.45, 0.94],
    },
  },
};

export function UseCasesSection() {
  const ref = useRef(null);
  const isInView = useInView(ref, { once: true, margin: '-100px' });
  const [activeCase, setActiveCase] = useState('healthcare');

  const activeUseCase = useCases.find(uc => uc.id === activeCase) || useCases[0];

  return (
    <section ref={ref} id="use-cases" className="py-24 md:py-32 relative">
      <div className="container mx-auto px-4 relative z-10">
        {/* Header */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={isInView ? { opacity: 1, y: 0 } : {}}
          transition={{ duration: 0.5 }}
          className="text-center mb-16"
        >
          <p className="text-primary text-sm font-medium mb-3">Use Cases</p>
          <h2 className="text-3xl md:text-4xl font-semibold mb-4 tracking-tight">
            Built for enterprise scale
          </h2>
          <p className="text-muted-foreground max-w-xl mx-auto">
            From healthcare to finance, organizations trust RMK to power their AI memory infrastructure.
          </p>
        </motion.div>

        <div className="grid lg:grid-cols-2 gap-12 items-start">
          {/* Use case tabs with staggered animation */}
          <motion.div
            className="space-y-2"
            variants={containerVariants}
            initial="hidden"
            animate={isInView ? "visible" : "hidden"}
          >
            {useCases.map((useCase) => (
              <motion.button
                key={useCase.id}
                variants={itemVariants}
                onClick={() => setActiveCase(useCase.id)}
                whileHover={{ scale: 1.01 }}
                whileTap={{ scale: 0.99 }}
                className={`w-full text-left p-4 rounded-lg transition-all duration-200 border ${
                  activeCase === useCase.id
                    ? 'bg-card/60 border-border'
                    : 'bg-transparent border-transparent hover:bg-card/30 hover:border-border/50'
                }`}
              >
                <div className="flex items-center gap-3">
                  <motion.div
                    className={`w-9 h-9 rounded-md flex items-center justify-center transition-colors ${
                      activeCase === useCase.id ? 'bg-primary/15' : 'bg-muted/50'
                    }`}
                    animate={{ 
                      scale: activeCase === useCase.id ? [1, 1.05, 1] : 1 
                    }}
                    transition={{ duration: 0.3 }}
                  >
                    <useCase.icon className={`w-4 h-4 ${activeCase === useCase.id ? 'text-primary' : 'text-muted-foreground'}`} />
                  </motion.div>
                  <div className="flex-1 min-w-0">
                    <h3 className="text-sm font-medium">{useCase.title}</h3>
                    <p className="text-xs text-muted-foreground">{useCase.subtitle}</p>
                  </div>
                </div>

                {activeCase === useCase.id && (
                  <motion.div
                    initial={{ opacity: 0, height: 0 }}
                    animate={{ opacity: 1, height: 'auto' }}
                    exit={{ opacity: 0, height: 0 }}
                    transition={{ duration: 0.3, ease: 'easeOut' }}
                    className="mt-3 pt-3 border-t border-border/50"
                  >
                    <p className="text-sm text-muted-foreground mb-3">{useCase.description}</p>
                    <div className="flex flex-wrap gap-2">
                      {useCase.stats.map((stat, i) => (
                        <motion.span
                          key={stat}
                          initial={{ opacity: 0, scale: 0.9 }}
                          animate={{ opacity: 1, scale: 1 }}
                          transition={{ delay: i * 0.1 }}
                          className="text-xs px-2 py-1 rounded bg-primary/10 text-primary"
                        >
                          {stat}
                        </motion.span>
                      ))}
                    </div>
                  </motion.div>
                )}
              </motion.button>
            ))}
          </motion.div>

          {/* Visualization with smooth transition */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={isInView ? { opacity: 1, y: 0 } : {}}
            transition={{ duration: 0.5, delay: 0.3 }}
            className="rounded-lg border border-border/50 bg-card/30 overflow-hidden"
          >
            <motion.div 
              className="p-4 border-b border-border/50"
              key={activeUseCase.id}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ duration: 0.3 }}
            >
              <h4 className="text-sm font-medium text-primary">
                {activeUseCase.title} Data Flow
              </h4>
              <p className="text-xs text-muted-foreground">Real-time memory synchronization</p>
            </motion.div>
            <DataFlow3D />
          </motion.div>
        </div>
      </div>
    </section>
  );
}
