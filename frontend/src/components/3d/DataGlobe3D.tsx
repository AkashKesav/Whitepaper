import { useRef, useMemo } from 'react';
import { Canvas, useFrame } from '@react-three/fiber';
import { EffectComposer, Bloom } from '@react-three/postprocessing';
import * as THREE from 'three';

const GLOBE_RADIUS = 2;

// Atmospheric glow shader (Fresnel effect)
function Atmosphere() {
  const ref = useRef<THREE.Mesh>(null);
  
  const atmosphereMaterial = useMemo(() => {
    return new THREE.ShaderMaterial({
      vertexShader: `
        varying vec3 vNormal;
        void main() {
          vNormal = normalize(normalMatrix * normal);
          gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
        }
      `,
      fragmentShader: `
        varying vec3 vNormal;
        void main() {
          float intensity = pow(0.65 - dot(vNormal, vec3(0.0, 0.0, 1.0)), 2.0);
          gl_FragColor = vec4(0.3, 0.6, 1.0, 1.0) * intensity * 0.8;
        }
      `,
      blending: THREE.AdditiveBlending,
      side: THREE.BackSide,
      transparent: true,
    });
  }, []);

  useFrame((state) => {
    if (ref.current) {
      ref.current.rotation.y = state.clock.elapsedTime * 0.08;
    }
  });

  return (
    <mesh ref={ref} scale={1.15} material={atmosphereMaterial}>
      <sphereGeometry args={[GLOBE_RADIUS, 64, 64]} />
    </mesh>
  );
}

// Inner glow for the globe
function InnerGlow() {
  const ref = useRef<THREE.Mesh>(null);
  
  useFrame((state) => {
    if (ref.current) {
      ref.current.rotation.y = state.clock.elapsedTime * 0.08;
    }
  });

  return (
    <mesh ref={ref} scale={1.02}>
      <sphereGeometry args={[GLOBE_RADIUS, 64, 64]} />
      <meshBasicMaterial 
        color="#0a1a2e" 
        transparent 
        opacity={0.95}
      />
    </mesh>
  );
}

// Dot matrix globe surface (like GitHub's globe)
function DotMatrixGlobe() {
  const ref = useRef<THREE.Points>(null);
  
  const { geometry, colors } = useMemo(() => {
    const positions: number[] = [];
    const colorArray: number[] = [];
    const dotCount = 2000;
    
    // Create dots distributed on sphere using fibonacci spiral
    const phi = Math.PI * (3 - Math.sqrt(5)); // golden angle
    
    for (let i = 0; i < dotCount; i++) {
      const y = 1 - (i / (dotCount - 1)) * 2;
      const radius = Math.sqrt(1 - y * y);
      const theta = phi * i;
      
      const x = Math.cos(theta) * radius;
      const z = Math.sin(theta) * radius;
      
      positions.push(
        x * GLOBE_RADIUS,
        y * GLOBE_RADIUS,
        z * GLOBE_RADIUS
      );
      
      // Vary colors slightly for visual interest
      const intensity = 0.3 + Math.random() * 0.4;
      colorArray.push(0, intensity * 0.8, intensity);
    }
    
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    geo.setAttribute('color', new THREE.Float32BufferAttribute(colorArray, 3));
    
    return { geometry: geo, colors: colorArray };
  }, []);

  useFrame((state) => {
    if (ref.current) {
      ref.current.rotation.y = state.clock.elapsedTime * 0.08;
    }
  });

  return (
    <points ref={ref} geometry={geometry}>
      <pointsMaterial 
        size={0.03} 
        vertexColors 
        transparent 
        opacity={0.7}
        sizeAttenuation
      />
    </points>
  );
}

// Subtle grid lines
function GlobeGrid() {
  const ref = useRef<THREE.LineSegments>(null);
  
  const geometry = useMemo(() => {
    const geo = new THREE.BufferGeometry();
    const positions: number[] = [];
    const r = GLOBE_RADIUS + 0.005;
    
    // Latitude lines - fewer, more elegant
    for (let lat = -60; lat <= 60; lat += 30) {
      const latRad = (lat * Math.PI) / 180;
      for (let lng = 0; lng < 360; lng += 3) {
        const lngRad = (lng * Math.PI) / 180;
        const lngRad2 = ((lng + 3) * Math.PI) / 180;
        
        positions.push(
          r * Math.cos(latRad) * Math.cos(lngRad),
          r * Math.sin(latRad),
          r * Math.cos(latRad) * Math.sin(lngRad)
        );
        positions.push(
          r * Math.cos(latRad) * Math.cos(lngRad2),
          r * Math.sin(latRad),
          r * Math.cos(latRad) * Math.sin(lngRad2)
        );
      }
    }
    
    // Longitude lines
    for (let lng = 0; lng < 360; lng += 30) {
      const lngRad = (lng * Math.PI) / 180;
      for (let lat = -80; lat < 80; lat += 3) {
        const latRad = (lat * Math.PI) / 180;
        const latRad2 = ((lat + 3) * Math.PI) / 180;
        
        positions.push(
          r * Math.cos(latRad) * Math.cos(lngRad),
          r * Math.sin(latRad),
          r * Math.cos(latRad) * Math.sin(lngRad)
        );
        positions.push(
          r * Math.cos(latRad2) * Math.cos(lngRad),
          r * Math.sin(latRad2),
          r * Math.cos(latRad2) * Math.sin(lngRad)
        );
      }
    }
    
    geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    return geo;
  }, []);

  useFrame((state) => {
    if (ref.current) {
      ref.current.rotation.y = state.clock.elapsedTime * 0.08;
    }
  });

  return (
    <lineSegments ref={ref} geometry={geometry}>
      <lineBasicMaterial color="#00d4ff" transparent opacity={0.08} />
    </lineSegments>
  );
}

// Data point with glow effect
function DataPoint({ position, delay, color = "#00d4ff" }: { position: [number, number, number]; delay: number; color?: string }) {
  const ref = useRef<THREE.Group>(null);
  const ringRef = useRef<THREE.Mesh>(null);
  
  useFrame((state) => {
    if (ref.current) {
      const scale = 0.8 + Math.sin(state.clock.elapsedTime * 2 + delay) * 0.3;
      ref.current.scale.setScalar(scale);
    }
    if (ringRef.current) {
      const ringScale = 1 + Math.sin(state.clock.elapsedTime * 1.5 + delay) * 0.5;
      ringRef.current.scale.setScalar(ringScale);
      (ringRef.current.material as THREE.MeshBasicMaterial).opacity = 0.6 - ringScale * 0.15;
    }
  });

  return (
    <group position={position}>
      <group ref={ref}>
        <mesh>
          <sphereGeometry args={[0.04, 16, 16]} />
          <meshBasicMaterial color={color} toneMapped={false} />
        </mesh>
      </group>
      {/* Pulse ring */}
      <mesh ref={ringRef} rotation={[Math.PI / 2, 0, 0]}>
        <ringGeometry args={[0.05, 0.08, 32]} />
        <meshBasicMaterial color={color} transparent opacity={0.4} side={THREE.DoubleSide} />
      </mesh>
    </group>
  );
}

// Create bezier arc between two points (more natural curve)
function createBezierArc(start: THREE.Vector3, end: THREE.Vector3, segments: number = 64): THREE.Vector3[] {
  // Calculate midpoint and lift it for the arc
  const mid = new THREE.Vector3().addVectors(start, end).multiplyScalar(0.5);
  const distance = start.distanceTo(end);
  
  // The arc height is proportional to the distance
  const arcHeight = GLOBE_RADIUS + 0.3 + distance * 0.15;
  mid.normalize().multiplyScalar(arcHeight);
  
  // Create quadratic bezier curve
  const curve = new THREE.QuadraticBezierCurve3(start, mid, end);
  return curve.getPoints(segments);
}

// Animated arc with trail effect
function AnimatedArc({ 
  start, 
  end, 
  color,
  speed = 0.3,
  delay = 0 
}: { 
  start: THREE.Vector3; 
  end: THREE.Vector3; 
  color: string;
  speed?: number;
  delay?: number;
}) {
  const particleRef = useRef<THREE.Mesh>(null);
  const groupRef = useRef<THREE.Group>(null);
  const progress = useRef(delay);
  
  const { arcPoints, baseLine, trailLine } = useMemo(() => {
    const points = createBezierArc(start, end, 64);
    const geo = new THREE.BufferGeometry().setFromPoints(points);
    const trailGeo = new THREE.BufferGeometry().setFromPoints(points);
    
    // Initialize color attribute for trail
    const colors = new Float32Array(points.length * 3);
    trailGeo.setAttribute('color', new THREE.BufferAttribute(colors, 3));
    
    const base = new THREE.Line(geo, new THREE.LineBasicMaterial({ color, transparent: true, opacity: 0.15 }));
    const trail = new THREE.Line(trailGeo, new THREE.LineBasicMaterial({ vertexColors: true, transparent: true }));
    
    return { arcPoints: points, baseLine: base, trailLine: trail };
  }, [start, end, color]);

  useFrame((state, delta) => {
    progress.current = (progress.current + delta * speed) % 1;
    
    // Update particle position along curve
    if (particleRef.current && arcPoints.length > 0) {
      const index = Math.floor(progress.current * (arcPoints.length - 1));
      const point = arcPoints[index];
      if (point) {
        particleRef.current.position.copy(point);
      }
    }
    
    // Animate trail visibility
    if (trailLine) {
      const positions = trailLine.geometry.attributes.position as THREE.BufferAttribute;
      const colorAttr = trailLine.geometry.attributes.color as THREE.BufferAttribute;
      
      for (let i = 0; i < positions.count; i++) {
        const t = i / positions.count;
        const diff = Math.abs(t - progress.current);
        const trailLength = 0.3;
        
        let alpha = 0;
        if (diff < trailLength && t <= progress.current) {
          alpha = 1 - (diff / trailLength);
        } else if (progress.current < trailLength && t > 1 - (trailLength - progress.current)) {
          alpha = 1 - ((1 - t + progress.current) / trailLength);
        }
        
        // Parse color and apply alpha
        const col = new THREE.Color(color);
        colorAttr.setXYZ(i, col.r * alpha, col.g * alpha, col.b * alpha);
      }
      colorAttr.needsUpdate = true;
    }
    
    // Rotate with globe
    if (groupRef.current) {
      groupRef.current.rotation.y = state.clock.elapsedTime * 0.08;
    }
  });

  return (
    <group ref={groupRef}>
      <primitive object={baseLine} />
      <primitive object={trailLine} />
      
      {/* Glowing particle */}
      <mesh ref={particleRef}>
        <sphereGeometry args={[0.05, 16, 16]} />
        <meshBasicMaterial color={color} toneMapped={false} />
      </mesh>
    </group>
  );
}

// Connection endpoint markers
function ConnectionMarker({ position, color }: { position: THREE.Vector3; color: string }) {
  const ref = useRef<THREE.Group>(null);
  
  useFrame((state) => {
    if (ref.current) {
      ref.current.rotation.y = state.clock.elapsedTime * 0.08;
    }
  });

  return (
    <group ref={ref}>
      <mesh position={position}>
        <sphereGeometry args={[0.06, 16, 16]} />
        <meshBasicMaterial color={color} toneMapped={false} />
      </mesh>
    </group>
  );
}

function GlobeScene() {
  // Generate data points on globe surface
  const dataPoints = useMemo(() => {
    const points: { pos: [number, number, number]; color: string }[] = [];
    const colors = ['#00d4ff', '#a855f7', '#22c55e', '#f59e0b'];
    
    for (let i = 0; i < 40; i++) {
      const lat = (Math.random() - 0.5) * 160;
      const lng = Math.random() * 360;
      const latRad = (lat * Math.PI) / 180;
      const lngRad = (lng * Math.PI) / 180;
      const r = GLOBE_RADIUS + 0.02;
      
      points.push({
        pos: [
          r * Math.cos(latRad) * Math.cos(lngRad),
          r * Math.sin(latRad),
          r * Math.cos(latRad) * Math.sin(lngRad),
        ],
        color: colors[Math.floor(Math.random() * colors.length)]
      });
    }
    return points;
  }, []);

  // Define connection routes (major data corridors)
  const connections = useMemo(() => {
    const routes = [
      // Americas to Europe
      { startLat: 40.7, startLng: -74, endLat: 51.5, endLng: -0.1, color: '#a855f7' },
      { startLat: 37.8, startLng: -122.4, endLat: 35.7, endLng: 139.7, color: '#00d4ff' },
      // Asia Pacific
      { startLat: 35.7, startLng: 139.7, endLat: -33.9, endLng: 151.2, color: '#22c55e' },
      { startLat: 22.3, startLng: 114.2, endLat: 1.3, endLng: 103.8, color: '#f59e0b' },
      // Europe to Asia
      { startLat: 51.5, startLng: -0.1, endLat: 55.8, endLng: 37.6, color: '#a855f7' },
      { startLat: 55.8, startLng: 37.6, endLat: 28.6, endLng: 77.2, color: '#00d4ff' },
      // Cross Atlantic
      { startLat: 40.7, startLng: -74, endLat: -23.5, endLng: -46.6, color: '#22c55e' },
      // Pacific routes
      { startLat: 37.8, startLng: -122.4, endLat: -33.9, endLng: 151.2, color: '#f59e0b' },
    ];
    
    return routes.map(({ startLat, startLng, endLat, endLng, color }) => {
      const r = GLOBE_RADIUS + 0.02;
      return {
        start: new THREE.Vector3(
          r * Math.cos((startLat * Math.PI) / 180) * Math.cos((startLng * Math.PI) / 180),
          r * Math.sin((startLat * Math.PI) / 180),
          r * Math.cos((startLat * Math.PI) / 180) * Math.sin((startLng * Math.PI) / 180)
        ),
        end: new THREE.Vector3(
          r * Math.cos((endLat * Math.PI) / 180) * Math.cos((endLng * Math.PI) / 180),
          r * Math.sin((endLat * Math.PI) / 180),
          r * Math.cos((endLat * Math.PI) / 180) * Math.sin((endLng * Math.PI) / 180)
        ),
        color
      };
    });
  }, []);

  return (
    <>
      {/* Atmospheric effects */}
      <Atmosphere />
      <InnerGlow />
      
      {/* Globe surface */}
      <DotMatrixGlobe />
      <GlobeGrid />
      
      {/* Data points */}
      <group rotation={[0, 0, 0]}>
        {dataPoints.map((point, i) => (
          <DataPoint key={i} position={point.pos} delay={i * 0.2} color={point.color} />
        ))}
      </group>
      
      {/* Animated data flows */}
      {connections.map((conn, i) => (
        <AnimatedArc
          key={`arc-${i}`}
          start={conn.start}
          end={conn.end}
          color={conn.color}
          speed={0.25 + (i % 3) * 0.1}
          delay={i * 0.12}
        />
      ))}
      
      {/* Connection markers */}
      {connections.map((conn, i) => (
        <ConnectionMarker key={`marker-start-${i}`} position={conn.start} color={conn.color} />
      ))}
    </>
  );
}

export function DataGlobe3D() {
  return (
    <div className="w-full h-[400px]">
      <Canvas 
        camera={{ position: [0, 0, 5.5], fov: 45 }}
        gl={{ antialias: true, alpha: true }}
      >
        <ambientLight intensity={0.3} />
        <pointLight position={[10, 10, 10]} intensity={0.4} />
        <pointLight position={[-10, -10, -10]} intensity={0.2} color="#a855f7" />
        
        <GlobeScene />
        
        {/* Post-processing bloom for glow effects */}
        <EffectComposer>
          <Bloom 
            intensity={0.8}
            luminanceThreshold={0.1}
            luminanceSmoothing={0.9}
            radius={0.8}
          />
        </EffectComposer>
      </Canvas>
    </div>
  );
}
