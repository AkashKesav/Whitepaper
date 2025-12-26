import { useRef, useMemo } from 'react';
import { Canvas, useFrame } from '@react-three/fiber';
import { Points, PointMaterial } from '@react-three/drei';
import * as THREE from 'three';

function NeuralNodes({ count = 200 }) {
  const ref = useRef<THREE.Points>(null);
  
  const positions = useMemo(() => {
    const pos = new Float32Array(count * 3);
    for (let i = 0; i < count; i++) {
      const theta = Math.random() * Math.PI * 2;
      const phi = Math.acos(2 * Math.random() - 1);
      const r = 2 + Math.random() * 1.5;
      
      pos[i * 3] = r * Math.sin(phi) * Math.cos(theta);
      pos[i * 3 + 1] = r * Math.sin(phi) * Math.sin(theta);
      pos[i * 3 + 2] = r * Math.cos(phi);
    }
    return pos;
  }, [count]);

  useFrame((state) => {
    if (ref.current) {
      ref.current.rotation.x = state.clock.elapsedTime * 0.05;
      ref.current.rotation.y = state.clock.elapsedTime * 0.08;
    }
  });

  return (
    <Points ref={ref} positions={positions} stride={3} frustumCulled={false}>
      <PointMaterial
        transparent
        color="#00d4ff"
        size={0.05}
        sizeAttenuation={true}
        depthWrite={false}
        blending={THREE.AdditiveBlending}
      />
    </Points>
  );
}

function Connections() {
  const ref = useRef<THREE.LineSegments>(null);
  
  const geometry = useMemo(() => {
    const geo = new THREE.BufferGeometry();
    const positions: number[] = [];
    const nodeCount = 40;
    const nodes: THREE.Vector3[] = [];
    
    for (let i = 0; i < nodeCount; i++) {
      const theta = Math.random() * Math.PI * 2;
      const phi = Math.acos(2 * Math.random() - 1);
      const r = 2 + Math.random() * 1;
      nodes.push(new THREE.Vector3(
        r * Math.sin(phi) * Math.cos(theta),
        r * Math.sin(phi) * Math.sin(theta),
        r * Math.cos(phi)
      ));
    }
    
    for (let i = 0; i < nodes.length; i++) {
      for (let j = i + 1; j < nodes.length; j++) {
        if (nodes[i].distanceTo(nodes[j]) < 1.5) {
          positions.push(nodes[i].x, nodes[i].y, nodes[i].z);
          positions.push(nodes[j].x, nodes[j].y, nodes[j].z);
        }
      }
    }
    
    geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    return geo;
  }, []);

  useFrame((state) => {
    if (ref.current) {
      ref.current.rotation.x = state.clock.elapsedTime * 0.05;
      ref.current.rotation.y = state.clock.elapsedTime * 0.08;
    }
  });

  return (
    <lineSegments ref={ref} geometry={geometry}>
      <lineBasicMaterial color="#00d4ff" transparent opacity={0.2} />
    </lineSegments>
  );
}

function CoreSphere() {
  const ref = useRef<THREE.Mesh>(null);

  useFrame((state) => {
    if (ref.current) {
      ref.current.scale.setScalar(1 + Math.sin(state.clock.elapsedTime * 2) * 0.05);
    }
  });

  return (
    <mesh ref={ref}>
      <sphereGeometry args={[0.5, 32, 32]} />
      <meshBasicMaterial color="#00d4ff" transparent opacity={0.3} />
    </mesh>
  );
}

export function NeuralNetwork3D() {
  return (
    <div className="absolute inset-0 z-0">
      <Canvas camera={{ position: [0, 0, 6], fov: 60 }}>
        <ambientLight intensity={0.5} />
        <CoreSphere />
        <NeuralNodes />
        <Connections />
      </Canvas>
    </div>
  );
}