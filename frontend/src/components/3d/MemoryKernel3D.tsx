import { useRef, useMemo, useState } from 'react';
import { Canvas, useFrame } from '@react-three/fiber';
import { EffectComposer, Bloom } from '@react-three/postprocessing';
import * as THREE from 'three';

// Central memory core - pulsating brain-like structure
function MemoryCore() {
  const meshRef = useRef<THREE.Mesh>(null);
  const glowRef = useRef<THREE.Mesh>(null);
  
  useFrame((state) => {
    if (meshRef.current) {
      const pulse = 1 + Math.sin(state.clock.elapsedTime * 2) * 0.05;
      meshRef.current.scale.setScalar(pulse);
      meshRef.current.rotation.y = state.clock.elapsedTime * 0.1;
      meshRef.current.rotation.x = Math.sin(state.clock.elapsedTime * 0.5) * 0.1;
    }
    if (glowRef.current) {
      const glowPulse = 1.2 + Math.sin(state.clock.elapsedTime * 1.5) * 0.1;
      glowRef.current.scale.setScalar(glowPulse);
    }
  });

  return (
    <group>
      {/* Outer glow */}
      <mesh ref={glowRef}>
        <icosahedronGeometry args={[0.8, 2]} />
        <meshBasicMaterial 
          color="#a855f7" 
          transparent 
          opacity={0.1}
          wireframe
        />
      </mesh>
      
      {/* Core */}
      <mesh ref={meshRef}>
        <icosahedronGeometry args={[0.5, 3]} />
        <meshStandardMaterial 
          color="#7c3aed"
          emissive="#a855f7"
          emissiveIntensity={0.5}
          roughness={0.3}
          metalness={0.8}
        />
      </mesh>
      
      {/* Inner energy */}
      <mesh scale={0.35}>
        <sphereGeometry args={[1, 32, 32]} />
        <meshBasicMaterial color="#c084fc" toneMapped={false} />
      </mesh>
    </group>
  );
}

// Memory node - represents entities/concepts
interface MemoryNodeProps {
  position: [number, number, number];
  size: number;
  color: string;
  delay: number;
  label: string;
}

function MemoryNode({ position, size, color, delay, label }: MemoryNodeProps) {
  const groupRef = useRef<THREE.Group>(null);
  const ringRef = useRef<THREE.Mesh>(null);
  const [hovered, setHovered] = useState(false);
  
  useFrame((state) => {
    if (groupRef.current) {
      // Floating animation
      groupRef.current.position.y = position[1] + Math.sin(state.clock.elapsedTime * 0.8 + delay) * 0.1;
      
      // Pulse on hover
      const targetScale = hovered ? 1.3 : 1;
      groupRef.current.scale.lerp(new THREE.Vector3(targetScale, targetScale, targetScale), 0.1);
    }
    if (ringRef.current) {
      ringRef.current.rotation.z = state.clock.elapsedTime + delay;
      const ringOpacity = 0.3 + Math.sin(state.clock.elapsedTime * 2 + delay) * 0.2;
      (ringRef.current.material as THREE.MeshBasicMaterial).opacity = ringOpacity;
    }
  });

  return (
    <group 
      ref={groupRef} 
      position={position}
      onPointerOver={() => setHovered(true)}
      onPointerOut={() => setHovered(false)}
    >
      {/* Node core */}
      <mesh>
        <sphereGeometry args={[size, 24, 24]} />
        <meshStandardMaterial 
          color={color}
          emissive={color}
          emissiveIntensity={hovered ? 0.8 : 0.4}
          roughness={0.2}
          metalness={0.6}
        />
      </mesh>
      
      {/* Orbital ring */}
      <mesh ref={ringRef}>
        <torusGeometry args={[size * 1.5, 0.02, 8, 32]} />
        <meshBasicMaterial color={color} transparent opacity={0.3} />
      </mesh>
      
      {/* Glow effect */}
      <mesh scale={1.2}>
        <sphereGeometry args={[size, 16, 16]} />
        <meshBasicMaterial color={color} transparent opacity={0.15} />
      </mesh>
    </group>
  );
}

// Synapse connection - flowing energy between nodes
interface SynapseProps {
  start: THREE.Vector3;
  end: THREE.Vector3;
  color: string;
  speed: number;
  delay: number;
}

function Synapse({ start, end, color, speed, delay }: SynapseProps) {
  const particlesRef = useRef<THREE.Points>(null);
  const progress = useRef(delay);
  
  const { curve, particleGeo } = useMemo(() => {
    // Create curved connection through center with some randomness
    const mid = new THREE.Vector3()
      .addVectors(start, end)
      .multiplyScalar(0.5);
    
    // Pull toward center for organic look
    const centerPull = 0.3 + Math.random() * 0.2;
    mid.multiplyScalar(centerPull);
    
    const curveObj = new THREE.QuadraticBezierCurve3(start, mid, end);
    
    // Particles along the curve
    const particleCount = 8;
    const positions = new Float32Array(particleCount * 3);
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    
    return { curve: curveObj, particleGeo: geo };
  }, [start, end]);
  
  const lineGeo = useMemo(() => {
    const points = curve.getPoints(30);
    return new THREE.BufferGeometry().setFromPoints(points);
  }, [curve]);

  useFrame((state, delta) => {
    progress.current = (progress.current + delta * speed) % 1;
    
    // Update particles
    if (particlesRef.current) {
      const positions = particlesRef.current.geometry.attributes.position as THREE.BufferAttribute;
      const particleCount = positions.count;
      
      for (let i = 0; i < particleCount; i++) {
        const t = (progress.current + i / particleCount) % 1;
        const point = curve.getPoint(t);
        positions.setXYZ(i, point.x, point.y, point.z);
      }
      positions.needsUpdate = true;
    }
  });

  const lineMesh = useMemo(() => {
    const material = new THREE.LineBasicMaterial({ color, transparent: true, opacity: 0.2 });
    return new THREE.Line(lineGeo, material);
  }, [lineGeo, color]);

  return (
    <group>
      {/* Connection line */}
      <primitive object={lineMesh} />
      
      {/* Flowing particles */}
      <points ref={particlesRef} geometry={particleGeo}>
        <pointsMaterial 
          color={color} 
          size={0.08} 
          transparent 
          opacity={0.9}
          sizeAttenuation
        />
      </points>
    </group>
  );
}

// Floating context particles
function ContextParticles() {
  const pointsRef = useRef<THREE.Points>(null);
  
  const { geometry, initialPositions } = useMemo(() => {
    const count = 200;
    const positions = new Float32Array(count * 3);
    const colors = new Float32Array(count * 3);
    const sizes = new Float32Array(count);
    const initPos: number[] = [];
    
    const colorOptions = [
      new THREE.Color('#a855f7'),
      new THREE.Color('#06b6d4'),
      new THREE.Color('#22c55e'),
      new THREE.Color('#f59e0b'),
    ];
    
    for (let i = 0; i < count; i++) {
      // Distribute in a smaller sphere shell to prevent cut-off
      const radius = 2 + Math.random() * 1.2;
      const theta = Math.random() * Math.PI * 2;
      const phi = Math.acos(2 * Math.random() - 1);
      
      const x = radius * Math.sin(phi) * Math.cos(theta);
      const y = radius * Math.sin(phi) * Math.sin(theta);
      const z = radius * Math.cos(phi);
      
      positions[i * 3] = x;
      positions[i * 3 + 1] = y;
      positions[i * 3 + 2] = z;
      
      initPos.push(x, y, z);
      
      const color = colorOptions[Math.floor(Math.random() * colorOptions.length)];
      colors[i * 3] = color.r;
      colors[i * 3 + 1] = color.g;
      colors[i * 3 + 2] = color.b;
      
      sizes[i] = 0.02 + Math.random() * 0.04;
    }
    
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    geo.setAttribute('color', new THREE.BufferAttribute(colors, 3));
    geo.setAttribute('size', new THREE.BufferAttribute(sizes, 1));
    
    return { geometry: geo, initialPositions: initPos };
  }, []);

  useFrame((state) => {
    if (pointsRef.current) {
      pointsRef.current.rotation.y = state.clock.elapsedTime * 0.03;
      pointsRef.current.rotation.x = Math.sin(state.clock.elapsedTime * 0.1) * 0.1;
      
      // Subtle position animation
      const positions = pointsRef.current.geometry.attributes.position as THREE.BufferAttribute;
      for (let i = 0; i < positions.count; i++) {
        const offset = Math.sin(state.clock.elapsedTime * 0.5 + i * 0.1) * 0.05;
        positions.setY(i, initialPositions[i * 3 + 1] + offset);
      }
      positions.needsUpdate = true;
    }
  });

  return (
    <points ref={pointsRef} geometry={geometry}>
      <pointsMaterial 
        size={0.04} 
        vertexColors 
        transparent 
        opacity={0.6}
        sizeAttenuation
      />
    </points>
  );
}

// Data stream rings
function DataRings() {
  const ring1Ref = useRef<THREE.Mesh>(null);
  const ring2Ref = useRef<THREE.Mesh>(null);
  const ring3Ref = useRef<THREE.Mesh>(null);

  useFrame((state) => {
    const t = state.clock.elapsedTime;
    if (ring1Ref.current) {
      ring1Ref.current.rotation.x = t * 0.3;
      ring1Ref.current.rotation.y = t * 0.2;
    }
    if (ring2Ref.current) {
      ring2Ref.current.rotation.x = -t * 0.2;
      ring2Ref.current.rotation.z = t * 0.25;
    }
    if (ring3Ref.current) {
      ring3Ref.current.rotation.y = t * 0.15;
      ring3Ref.current.rotation.z = -t * 0.3;
    }
  });

  return (
    <group>
      <mesh ref={ring1Ref}>
        <torusGeometry args={[1.6, 0.015, 4, 64]} />
        <meshBasicMaterial color="#a855f7" transparent opacity={0.3} />
      </mesh>
      <mesh ref={ring2Ref}>
        <torusGeometry args={[1.85, 0.012, 4, 64]} />
        <meshBasicMaterial color="#06b6d4" transparent opacity={0.25} />
      </mesh>
      <mesh ref={ring3Ref}>
        <torusGeometry args={[2.1, 0.01, 4, 64]} />
        <meshBasicMaterial color="#22c55e" transparent opacity={0.2} />
      </mesh>
    </group>
  );
}

function MemoryScene() {
  // Memory nodes positioned closer to the core
  const nodes = useMemo(() => [
    { position: [1.3, 0.4, 0.6] as [number, number, number], size: 0.12, color: '#06b6d4', label: 'Entities' },
    { position: [-1.1, 0.6, 0.9] as [number, number, number], size: 0.14, color: '#22c55e', label: 'Patterns' },
    { position: [0.4, -1.0, 1.1] as [number, number, number], size: 0.1, color: '#f59e0b', label: 'Context' },
    { position: [-0.9, -0.5, -1.0] as [number, number, number], size: 0.13, color: '#a855f7', label: 'Insights' },
    { position: [1.0, 0.15, -1.0] as [number, number, number], size: 0.11, color: '#ec4899', label: 'Relations' },
    { position: [-0.2, 1.1, -0.6] as [number, number, number], size: 0.1, color: '#06b6d4', label: 'Memory' },
    { position: [0.6, -0.6, -1.3] as [number, number, number], size: 0.09, color: '#22c55e', label: 'Facts' },
    { position: [-1.3, 0.15, -0.4] as [number, number, number], size: 0.12, color: '#f59e0b', label: 'Events' },
  ], []);

  // Synapses connecting nodes to core and each other
  const synapses = useMemo(() => {
    const connections: { start: THREE.Vector3; end: THREE.Vector3; color: string }[] = [];
    const center = new THREE.Vector3(0, 0, 0);
    
    // Connect each node to center
    nodes.forEach((node) => {
      connections.push({
        start: new THREE.Vector3(...node.position),
        end: center,
        color: node.color
      });
    });
    
    // Connect some nodes to each other
    const pairs = [[0, 1], [2, 3], [4, 5], [6, 7], [0, 4], [1, 5], [2, 6], [3, 7]];
    pairs.forEach(([a, b]) => {
      connections.push({
        start: new THREE.Vector3(...nodes[a].position),
        end: new THREE.Vector3(...nodes[b].position),
        color: nodes[a].color
      });
    });
    
    return connections;
  }, [nodes]);

  return (
    <>
      {/* Core memory kernel */}
      <MemoryCore />
      
      {/* Orbital data rings */}
      <DataRings />
      
      {/* Memory nodes */}
      {nodes.map((node, i) => (
        <MemoryNode 
          key={i}
          position={node.position}
          size={node.size}
          color={node.color}
          delay={i * 0.5}
          label={node.label}
        />
      ))}
      
      {/* Synapse connections */}
      {synapses.map((synapse, i) => (
        <Synapse
          key={i}
          start={synapse.start}
          end={synapse.end}
          color={synapse.color}
          speed={0.3 + (i % 4) * 0.1}
          delay={i * 0.15}
        />
      ))}
      
      {/* Ambient context particles */}
      <ContextParticles />
    </>
  );
}

export function MemoryKernel3D() {
  return (
    <div className="relative w-full h-[450px]">
      {/* Blurred edge overlay */}
      <div 
        className="absolute inset-0 pointer-events-none z-10"
        style={{
          boxShadow: 'inset 0 0 50px 25px hsl(var(--background))',
        }}
      />
      
      <Canvas 
        camera={{ position: [0, 0, 4.5], fov: 50 }}
        gl={{ antialias: true, alpha: true }}
        style={{ background: 'transparent' }}
      >
        <ambientLight intensity={0.4} />
        <pointLight position={[5, 5, 5]} intensity={0.6} color="#ffffff" />
        <pointLight position={[-5, -5, 5]} intensity={0.3} color="#a855f7" />
        <pointLight position={[0, 0, -5]} intensity={0.2} color="#06b6d4" />
        
        <MemoryScene />
        
        <EffectComposer>
          <Bloom 
            intensity={0.6}
            luminanceThreshold={0.2}
            luminanceSmoothing={0.9}
            radius={0.8}
          />
        </EffectComposer>
      </Canvas>
    </div>
  );
}
