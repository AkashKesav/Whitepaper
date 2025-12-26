import { useRef, useMemo } from 'react';
import { Canvas, useFrame } from '@react-three/fiber';
import { Points, PointMaterial } from '@react-three/drei';
import * as THREE from 'three';

function ParticleField({ count = 3000, color = '#00d4ff', size = 0.015, speed = 0.02 }) {
  const ref = useRef<THREE.Points>(null);

  const positions = useMemo(() => {
    const pos = new Float32Array(count * 3);
    for (let i = 0; i < count; i++) {
      pos[i * 3] = (Math.random() - 0.5) * 20;
      pos[i * 3 + 1] = (Math.random() - 0.5) * 20;
      pos[i * 3 + 2] = (Math.random() - 0.5) * 20;
    }
    return pos;
  }, [count]);

  useFrame((state) => {
    if (ref.current) {
      ref.current.rotation.x = state.clock.elapsedTime * speed * 0.5;
      ref.current.rotation.y = state.clock.elapsedTime * speed;
    }
  });

  return (
    <Points ref={ref} positions={positions} stride={3} frustumCulled={false}>
      <PointMaterial
        transparent
        color={color}
        size={size}
        sizeAttenuation={true}
        depthWrite={false}
        blending={THREE.AdditiveBlending}
      />
    </Points>
  );
}

function FloatingOrbs() {
  const orbs = useMemo(() => {
    return Array.from({ length: 8 }, (_, i) => ({
      position: [
        (Math.random() - 0.5) * 8,
        (Math.random() - 0.5) * 8,
        (Math.random() - 0.5) * 8,
      ] as [number, number, number],
      scale: 0.1 + Math.random() * 0.3,
      speed: 0.5 + Math.random() * 1,
      color: ['#00d4ff', '#a855f7', '#22c55e', '#f59e0b'][i % 4],
    }));
  }, []);

  return (
    <>
      {orbs.map((orb, i) => (
        <FloatingOrb key={i} {...orb} delay={i * 0.5} />
      ))}
    </>
  );
}

function FloatingOrb({ 
  position, 
  scale, 
  speed, 
  color, 
  delay 
}: { 
  position: [number, number, number]; 
  scale: number; 
  speed: number; 
  color: string;
  delay: number;
}) {
  const ref = useRef<THREE.Mesh>(null);

  useFrame((state) => {
    if (ref.current) {
      ref.current.position.y = position[1] + Math.sin(state.clock.elapsedTime * speed + delay) * 0.5;
      ref.current.position.x = position[0] + Math.cos(state.clock.elapsedTime * speed * 0.5 + delay) * 0.3;
    }
  });

  return (
    <mesh ref={ref} position={position} scale={scale}>
      <sphereGeometry args={[1, 16, 16]} />
      <meshStandardMaterial
        color={color}
        emissive={color}
        emissiveIntensity={0.5}
        transparent
        opacity={0.6}
      />
    </mesh>
  );
}

function ConnectingLines() {
  const ref = useRef<THREE.LineSegments>(null);

  const geometry = useMemo(() => {
    const geo = new THREE.BufferGeometry();
    const positions: number[] = [];
    const count = 50;

    for (let i = 0; i < count; i++) {
      const x1 = (Math.random() - 0.5) * 10;
      const y1 = (Math.random() - 0.5) * 10;
      const z1 = (Math.random() - 0.5) * 10;
      const x2 = x1 + (Math.random() - 0.5) * 2;
      const y2 = y1 + (Math.random() - 0.5) * 2;
      const z2 = z1 + (Math.random() - 0.5) * 2;

      positions.push(x1, y1, z1, x2, y2, z2);
    }

    geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    return geo;
  }, []);

  useFrame((state) => {
    if (ref.current) {
      ref.current.rotation.y = state.clock.elapsedTime * 0.02;
    }
  });

  return (
    <lineSegments ref={ref} geometry={geometry}>
      <lineBasicMaterial color="#a855f7" transparent opacity={0.15} />
    </lineSegments>
  );
}

export function AbstractParticles3D({ className = '' }: { className?: string }) {
  return (
    <div className={`absolute inset-0 z-0 ${className}`}>
      <Canvas camera={{ position: [0, 0, 8], fov: 60 }}>
        <ambientLight intensity={0.2} />
        <ParticleField count={2000} color="#00d4ff" size={0.01} speed={0.01} />
        <ParticleField count={1000} color="#a855f7" size={0.015} speed={0.015} />
        <ParticleField count={500} color="#22c55e" size={0.02} speed={0.008} />
        <FloatingOrbs />
        <ConnectingLines />
      </Canvas>
    </div>
  );
}