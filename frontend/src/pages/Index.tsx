import { useRef } from 'react';
import { useScroll, useTransform } from 'framer-motion';
import { Navbar } from '@/components/landing/Navbar';
import { HeroSection } from '@/components/landing/HeroSection';
import { FeaturesSection } from '@/components/landing/FeaturesSection';
import { KnowledgeGraphSection } from '@/components/landing/KnowledgeGraphSection';
import { UseCasesSection } from '@/components/landing/UseCasesSection';
import { FooterSection } from '@/components/landing/FooterSection';
import { StackedDashboards } from '@/components/landing/StackedDashboards';

const Index = () => {
  const dashboardSectionRef = useRef<HTMLDivElement>(null);
  
  // Track scroll through hero + features section for Apple-style dashboard animation
  const { scrollYProgress } = useScroll({
    target: dashboardSectionRef,
    offset: ['start start', 'end start'],
  });

  return (
    <main className="min-h-screen bg-background text-foreground overflow-x-hidden">
      <Navbar />
      
      {/* Dashboard visibility zone - hero + features */}
      <div ref={dashboardSectionRef}>
        <StackedDashboards scrollProgress={scrollYProgress} />
        <HeroSection />
        <FeaturesSection />
      </div>
      
      {/* Rest of the page - no dashboard background */}
      <KnowledgeGraphSection />
      <UseCasesSection />
      <FooterSection />
    </main>
  );
};

export default Index;