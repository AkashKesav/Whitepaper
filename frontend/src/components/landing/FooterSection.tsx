import { motion, useInView } from 'framer-motion';
import { useRef } from 'react';
import { Button } from '@/components/ui/button';
import { ArrowRight, Github, Twitter, Linkedin } from 'lucide-react';

const footerLinks = [
  {
    title: 'Product',
    links: ['Features', 'Pricing', 'Enterprise', 'Changelog'],
  },
  {
    title: 'Developers',
    links: ['Documentation', 'API Reference', 'SDKs', 'Examples'],
  },
  {
    title: 'Company',
    links: ['About', 'Blog', 'Careers', 'Contact'],
  },
  {
    title: 'Legal',
    links: ['Privacy', 'Terms', 'Security'],
  },
];

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.05,
      delayChildren: 0.1,
    },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 10 },
  visible: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.3 },
  },
};

export function FooterSection() {
  const ref = useRef(null);
  const isInView = useInView(ref, { once: true, margin: '-50px' });

  return (
    <footer ref={ref} id="pricing" className="relative">
      {/* CTA Section with fade-in */}
      <section className="py-24 md:py-32 relative">
        <div className="absolute inset-0 bg-gradient-to-b from-transparent via-primary/3 to-transparent" />

        <div className="container mx-auto px-4 relative z-10">
          <motion.div
            initial={{ opacity: 0, y: 30 }}
            animate={isInView ? { opacity: 1, y: 0 } : {}}
            transition={{ duration: 0.6, ease: 'easeOut' }}
            className="max-w-2xl mx-auto text-center"
          >
            <h2 className="text-3xl md:text-4xl font-semibold mb-4 tracking-tight">
              Ready to give your AI a memory?
            </h2>
            <p className="text-muted-foreground mb-8">
              Join hundreds of teams already using Reflective Memory Kernel 
              to build smarter, more context-aware AI.
            </p>
            <motion.div 
              className="flex flex-col sm:flex-row gap-3 justify-center"
              initial={{ opacity: 0, y: 20 }}
              animate={isInView ? { opacity: 1, y: 0 } : {}}
              transition={{ duration: 0.5, delay: 0.2 }}
            >
              <Button size="lg" className="group h-11 px-6 text-sm font-medium">
                Start free trial
                <ArrowRight className="w-4 h-4 ml-2 group-hover:translate-x-0.5 transition-transform" />
              </Button>
              <Button variant="outline" size="lg" className="h-11 px-6 text-sm font-medium">
                Schedule demo
              </Button>
            </motion.div>
          </motion.div>
        </div>
      </section>

      {/* Footer links with staggered animation */}
      <div className="border-t border-border/50 py-12">
        <div className="container mx-auto px-4">
          <motion.div 
            className="grid grid-cols-2 md:grid-cols-5 gap-8 mb-12"
            variants={containerVariants}
            initial="hidden"
            animate={isInView ? "visible" : "hidden"}
          >
            {/* Brand */}
            <motion.div variants={itemVariants} className="col-span-2 md:col-span-1">
              <div className="flex items-center gap-2 mb-4">
                <div className="w-7 h-7 rounded-md bg-primary/20 flex items-center justify-center">
                  <span className="font-semibold text-primary text-sm">R</span>
                </div>
                <span className="font-semibold text-sm">RMK</span>
              </div>
              <p className="text-xs text-muted-foreground mb-4 leading-relaxed">
                Persistent, entity-centric AI memory.
              </p>
              <div className="flex gap-2">
                {[Github, Twitter, Linkedin].map((Icon, i) => (
                  <motion.a
                    key={i}
                    href="#"
                    whileHover={{ scale: 1.1 }}
                    whileTap={{ scale: 0.95 }}
                    className="w-8 h-8 rounded-md bg-muted/50 flex items-center justify-center hover:bg-muted transition-colors text-muted-foreground hover:text-foreground"
                  >
                    <Icon className="w-4 h-4" />
                  </motion.a>
                ))}
              </div>
            </motion.div>

            {/* Links */}
            {footerLinks.map((section) => (
              <motion.div key={section.title} variants={itemVariants}>
                <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-3">{section.title}</h4>
                <ul className="space-y-2">
                  {section.links.map((link) => (
                    <li key={link}>
                      <motion.a
                        href="#"
                        whileHover={{ x: 2 }}
                        className="text-sm text-muted-foreground hover:text-foreground transition-colors inline-block"
                      >
                        {link}
                      </motion.a>
                    </li>
                  ))}
                </ul>
              </motion.div>
            ))}
          </motion.div>

          {/* Bottom bar */}
          <motion.div 
            className="pt-8 border-t border-border/50 flex flex-col md:flex-row justify-between items-center gap-4"
            initial={{ opacity: 0 }}
            animate={isInView ? { opacity: 1 } : {}}
            transition={{ duration: 0.5, delay: 0.4 }}
          >
            <p className="text-xs text-muted-foreground">
              © 2024 Reflective Memory Kernel. All rights reserved.
            </p>
            <p className="text-xs text-muted-foreground">
              hello@rmk.ai
            </p>
          </motion.div>
        </div>
      </div>
    </footer>
  );
}
