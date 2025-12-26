import { useRef, useMemo } from 'react';
import { Canvas, useFrame } from '@react-three/fiber';
import { EffectComposer, Bloom } from '@react-three/postprocessing';
import * as THREE from 'three';

// Flowing helix strand
function HelixStrand({ 
  color, 
  radius, 
  offset, 
  speed,
  direction = 1 
}: { 
  color: string; 
  radius: number; 
  offset: number; 
  speed: number;
  direction?: number;
}) {
  const pointsRef = useRef<THREE.Points>(null);
  
  const geometry = useMemo(() => {
    const count = 80;
    const positions = new Float32Array(count * 3);
    const sizes = new Float32Array(count);
    
    for (let i = 0; i < count; i++) {
      const t = (i / count) * Math.PI * 4;
      positions[i * 3] = Math.cos(t + offset) * radius;
      positions[i * 3 + 1] = (i / count) * 5 - 2.5;
      positions[i * 3 + 2] = Math.sin(t + offset) * radius;
      sizes[i] = 0.04 + Math.sin(i * 0.2) * 0.02;
    }
    
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    geo.setAttribute('size', new THREE.BufferAttribute(sizes, 1));
    return geo;
  }, [radius, offset]);

  useFrame((state) => {
    if (pointsRef.current) {
      const positions = pointsRef.current.geometry.attributes.position as THREE.BufferAttribute;
      const count = positions.count;
      const time = state.clock.elapsedTime * speed * direction;
      
      for (let i = 0; i < count; i++) {
        const t = (i / count) * Math.PI * 4 + time;
        positions.setX(i, Math.cos(t + offset) * radius);
        positions.setZ(i, Math.sin(t + offset) * radius);
      }
      positions.needsUpdate = true;
    }
  });

  return (
    <points ref={pointsRef} geometry={geometry}>
      <pointsMaterial 
        color={color} 
        size={0.08} 
        transparent 
        opacity={0.9}
        sizeAttenuation
      />
    </points>
  );
}

// Central energy core
function EnergyCore() {
  const coreRef = useRef<THREE.Mesh>(null);
  const pulseRef = useRef<THREE.Mesh>(null);
  
  useFrame((state) => {
    const t = state.clock.elapsedTime;
    if (coreRef.current) {
      coreRef.current.rotation.y = t * 0.5;
      coreRef.current.rotation.x = Math.sin(t * 0.3) * 0.2;
    }
    if (pulseRef.current) {
      const scale = 1 + Math.sin(t * 2) * 0.15;
      pulseRef.current.scale.setScalar(scale);
      (pulseRef.current.material as THREE.MeshBasicMaterial).opacity = 0.3 - Math.sin(t * 2) * 0.1;
    }
  });

  return (
    <group>
      {/* Outer pulse */}
      <mesh ref={pulseRef}>
        <octahedronGeometry args={[0.6, 0]} />
        <meshBasicMaterial color="#a855f7" transparent opacity={0.2} wireframe />
      </mesh>
      
      {/* Inner core */}
      <mesh ref={coreRef}>
        <octahedronGeometry args={[0.35, 1]} />
        <meshStandardMaterial 
          color="#7c3aed"
          emissive="#a855f7"
          emissiveIntensity={0.6}
          roughness={0.2}
          metalness={0.8}
        />
      </mesh>
      
      {/* Glowing center */}
      <mesh>
        <sphereGeometry args={[0.2, 16, 16]} />
        <meshBasicMaterial color="#c084fc" toneMapped={false} />
      </mesh>
    </group>
  );
}

// Orbiting data nodes
function DataNodes() {
  const groupRef = useRef<THREE.Group>(null);
  
  const nodes = useMemo(() => {
    const items: { position: THREE.Vector3; color: string; size: number; speed: number }[] = [];
    const colors = ['#06b6d4', '#22c55e', '#f59e0b', '#ec4899'];
    
    for (let i = 0; i < 12; i++) {
      const angle = (i / 12) * Math.PI * 2;
      const radius = 1.5 + Math.random() * 0.5;
      const y = (Math.random() - 0.5) * 3;
      
      items.push({
        position: new THREE.Vector3(
          Math.cos(angle) * radius,
          y,
          Math.sin(angle) * radius
        ),
        color: colors[i % colors.length],
        size: 0.08 + Math.random() * 0.05,
        speed: 0.3 + Math.random() * 0.4
      });
    }
    return items;
  }, []);

  useFrame((state) => {
    if (groupRef.current) {
      groupRef.current.rotation.y = state.clock.elapsedTime * 0.2;
    }
  });

  return (
    <group ref={groupRef}>
      {nodes.map((node, i) => (
        <OrbitingNode key={i} {...node} index={i} />
      ))}
    </group>
  );
}

function OrbitingNode({ 
  position, 
  color, 
  size, 
  speed,
  index 
}: { 
  position: THREE.Vector3; 
  color: string; 
  size: number; 
  speed: number;
  index: number;
}) {
  const meshRef = useRef<THREE.Mesh>(null);
  const trailRef = useRef<THREE.Points>(null);
  
  const trailGeo = useMemo(() => {
    const count = 20;
    const positions = new Float32Array(count * 3);
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    return geo;
  }, []);

  useFrame((state) => {
    const t = state.clock.elapsedTime * speed + index;
    const radius = position.length();
    
    if (meshRef.current) {
      meshRef.current.position.x = Math.cos(t) * radius;
      meshRef.current.position.z = Math.sin(t) * radius;
      meshRef.current.position.y = position.y + Math.sin(t * 2) * 0.2;
      
      // Update trail
      if (trailRef.current) {
        const positions = trailRef.current.geometry.attributes.position as THREE.BufferAttribute;
        for (let i = positions.count - 1; i > 0; i--) {
          positions.setXYZ(
            i,
            positions.getX(i - 1),
            positions.getY(i - 1),
            positions.getZ(i - 1)
          );
        }
        positions.setXYZ(0, meshRef.current.position.x, meshRef.current.position.y, meshRef.current.position.z);
        positions.needsUpdate = true;
      }
    }
  });

  return (
    <group>
      <mesh ref={meshRef} position={position}>
        <sphereGeometry args={[size, 12, 12]} />
        <meshBasicMaterial color={color} toneMapped={false} />
      </mesh>
      <points ref={trailRef} geometry={trailGeo}>
        <pointsMaterial color={color} size={0.03} transparent opacity={0.5} sizeAttenuation />
      </points>
    </group>
  );
}

// Floating particles background
function FloatingParticles() {
  const pointsRef = useRef<THREE.Points>(null);
  
  const geometry = useMemo(() => {
    const count = 150;
    const positions = new Float32Array(count * 3);
    const colors = new Float32Array(count * 3);
    
    const colorOptions = [
      new THREE.Color('#a855f7'),
      new THREE.Color('#06b6d4'),
      new THREE.Color('#22c55e'),
    ];
    
    for (let i = 0; i < count; i++) {
      const radius = 2.5 + Math.random() * 2;
      const theta = Math.random() * Math.PI * 2;
      const phi = Math.acos(2 * Math.random() - 1);
      
      positions[i * 3] = radius * Math.sin(phi) * Math.cos(theta);
      positions[i * 3 + 1] = radius * Math.sin(phi) * Math.sin(theta);
      positions[i * 3 + 2] = radius * Math.cos(phi);
      
      const color = colorOptions[Math.floor(Math.random() * colorOptions.length)];
      colors[i * 3] = color.r;
      colors[i * 3 + 1] = color.g;
      colors[i * 3 + 2] = color.b;
    }
    
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    geo.setAttribute('color', new THREE.BufferAttribute(colors, 3));
    return geo;
  }, []);

  useFrame((state) => {
    if (pointsRef.current) {
      pointsRef.current.rotation.y = state.clock.elapsedTime * 0.05;
      pointsRef.current.rotation.x = Math.sin(state.clock.elapsedTime * 0.1) * 0.1;
    }
  });

  return (
    <points ref={pointsRef} geometry={geometry}>
      <pointsMaterial size={0.03} vertexColors transparent opacity={0.6} sizeAttenuation />
    </points>
  );
}

function DataFlowScene() {
  return (
    <>
      {/* Central core */}
      <EnergyCore />
      
      {/* DNA-like helix strands */}
      <HelixStrand color="#a855f7" radius={0.8} offset={0} speed={0.8} direction={1} />
      <HelixStrand color="#06b6d4" radius={0.8} offset={Math.PI} speed={0.8} direction={1} />
      <HelixStrand color="#22c55e" radius={1.1} offset={Math.PI / 2} speed={0.6} direction={-1} />
      <HelixStrand color="#f59e0b" radius={1.1} offset={Math.PI * 1.5} speed={0.6} direction={-1} />
      
      {/* Orbiting nodes */}
      <DataNodes />
      
      {/* Background particles */}
      <FloatingParticles />
    </>
  );
}

export function DataFlow3D() {
  return (
    <div className="w-full h-[400px]">
      <Canvas 
        camera={{ position: [0, 1, 5], fov: 50 }}
        gl={{ antialias: true, alpha: true }}
      >
        <ambientLight intensity={0.4} />
        <pointLight position={[5, 5, 5]} intensity={0.5} />
        <pointLight position={[-5, -5, 5]} intensity={0.3} color="#a855f7" />
        
        <DataFlowScene />
        
        <EffectComposer>
          <Bloom 
            intensity={0.7}
            luminanceThreshold={0.15}
            luminanceSmoothing={0.9}
            radius={0.8}
          />
        </EffectComposer>
      </Canvas>
    </div>
  );
}
