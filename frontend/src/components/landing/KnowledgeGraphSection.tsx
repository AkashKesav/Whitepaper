import { motion, useInView } from 'framer-motion';
import { useRef } from 'react';
import { MemoryKernel3D } from '@/components/3d/MemoryKernel3D';

const features = [
  {
    title: 'Entity Relationships',
    description: 'See how entities connect and influence each other',
  },
  {
    title: 'Pattern Detection',
    description: 'Discover recurring themes and behaviors automatically',
  },
  {
    title: 'Insight Synthesis',
    description: 'Watch as the system generates new insights from connections',
  },
];

const listVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.15,
      delayChildren: 0.3,
    },
  },
};

const listItemVariants = {
  hidden: { opacity: 0, x: -15 },
  visible: {
    opacity: 1,
    x: 0,
    transition: {
      duration: 0.4,
      ease: 'easeOut',
    },
  },
};

export function KnowledgeGraphSection() {
  const ref = useRef(null);
  const isInView = useInView(ref, { once: true, margin: '-100px' });

  return (
    <section ref={ref} id="demo" className="py-24 md:py-32 relative">
      {/* Subtle background gradient */}
      <div className="absolute inset-0 bg-gradient-to-b from-transparent via-muted/20 to-transparent" />
      
      <div className="container mx-auto px-4 relative z-10">
        <div className="grid lg:grid-cols-2 gap-12 lg:gap-16 items-center">
          {/* Text content with staggered animation */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={isInView ? { opacity: 1, y: 0 } : {}}
            transition={{ duration: 0.5 }}
          >
            <p className="text-primary text-sm font-medium mb-3">Interactive Demo</p>
            <h2 className="text-3xl md:text-4xl font-semibold mb-4 tracking-tight">
              Explore your Knowledge Graph
            </h2>
            <p className="text-muted-foreground mb-8 leading-relaxed">
              Visualize the intricate web of entities, insights, and patterns that power your AI's memory. 
              Hover over nodes to explore connections.
            </p>

            <motion.div 
              className="space-y-4"
              variants={listVariants}
              initial="hidden"
              animate={isInView ? "visible" : "hidden"}
            >
              {features.map((item) => (
                <motion.div
                  key={item.title}
                  variants={listItemVariants}
                  className="flex items-start gap-3 group"
                >
                  <motion.div 
                    className="w-1.5 h-1.5 rounded-full bg-primary mt-2 flex-shrink-0"
                    whileHover={{ scale: 1.5 }}
                  />
                  <div>
                    <h4 className="text-sm font-medium text-foreground group-hover:text-primary transition-colors">{item.title}</h4>
                    <p className="text-sm text-muted-foreground">{item.description}</p>
                  </div>
                </motion.div>
              ))}
            </motion.div>
          </motion.div>

          {/* 3D Knowledge Graph with scale-in animation */}
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={isInView ? { opacity: 1, scale: 1 } : {}}
            transition={{ duration: 0.6, delay: 0.2, ease: 'easeOut' }}
            className="overflow-hidden"
          >
            <MemoryKernel3D />
          </motion.div>
        </div>
      </div>
    </section>
  );
}
