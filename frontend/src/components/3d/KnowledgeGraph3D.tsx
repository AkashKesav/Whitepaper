import { useRef, useMemo, useState } from 'react';
import { Canvas, useFrame, ThreeEvent } from '@react-three/fiber';
import { Text } from '@react-three/drei';
import * as THREE from 'three';

interface NodeData {
  id: string;
  label: string;
  type: 'entity' | 'user' | 'insight' | 'pattern' | 'alert';
  basePosition: [number, number, number];
  connections: string[];
}

const nodeColors: Record<string, string> = {
  entity: '#00d4ff',
  user: '#a855f7',
  insight: '#22c55e',
  pattern: '#f59e0b',
  alert: '#ef4444',
};

interface NodeRefs {
  [key: string]: THREE.Vector3;
}

function GraphNode({ 
  node, 
  isHovered,
  onHover,
  onUnhover,
  onPositionUpdate,
}: { 
  node: NodeData;
  isHovered: boolean;
  onHover: () => void;
  onUnhover: () => void;
  onPositionUpdate: (id: string, position: THREE.Vector3) => void;
}) {
  const meshRef = useRef<THREE.Mesh>(null);
  const groupRef = useRef<THREE.Group>(null);
  const scale = isHovered ? 1.4 : 1;
  const [x, y, z] = node.basePosition;

  useFrame((state) => {
    if (groupRef.current) {
      const floatY = Math.sin(state.clock.elapsedTime * 0.8 + x * 2) * 0.08;
      const floatX = Math.cos(state.clock.elapsedTime * 0.5 + z * 2) * 0.05;
      groupRef.current.position.set(x + floatX, y + floatY, z);
      
      // Report current world position
      onPositionUpdate(node.id, groupRef.current.position.clone());
    }
  });

  return (
    <group ref={groupRef} position={[x, y, z]}>
      <mesh
        ref={meshRef}
        scale={scale}
        onPointerOver={(e: ThreeEvent<PointerEvent>) => {
          e.stopPropagation();
          onHover();
          document.body.style.cursor = 'pointer';
        }}
        onPointerOut={() => {
          onUnhover();
          document.body.style.cursor = 'auto';
        }}
      >
        <sphereGeometry args={[0.12, 24, 24]} />
        <meshStandardMaterial
          color={nodeColors[node.type]}
          emissive={nodeColors[node.type]}
          emissiveIntensity={isHovered ? 1 : 0.4}
          roughness={0.3}
          metalness={0.8}
        />
      </mesh>
      
      {/* Glow ring */}
      <mesh scale={isHovered ? 1.8 : 1.3}>
        <ringGeometry args={[0.14, 0.18, 32]} />
        <meshBasicMaterial 
          color={nodeColors[node.type]} 
          transparent 
          opacity={isHovered ? 0.6 : 0.2}
          side={THREE.DoubleSide}
        />
      </mesh>

      {/* Label */}
      {isHovered && (
        <Text
          position={[0, 0.35, 0]}
          fontSize={0.1}
          color="white"
          anchorX="center"
          anchorY="middle"
          outlineWidth={0.01}
          outlineColor="#000000"
        >
          {node.label}
        </Text>
      )}
    </group>
  );
}

function AnimatedConnection({ 
  startPos, 
  endPos, 
  color,
  isHighlighted 
}: { 
  startPos: THREE.Vector3; 
  endPos: THREE.Vector3; 
  color: string;
  isHighlighted: boolean;
}) {
  const lineRef = useRef<THREE.Line>(null);
  const particleRef = useRef<THREE.Mesh>(null);
  const progress = useRef(Math.random());

  const lineGeometry = useMemo(() => {
    const geometry = new THREE.BufferGeometry();
    const positions = new Float32Array(6);
    geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    return geometry;
  }, []);

  const lineMaterial = useMemo(() => {
    return new THREE.LineBasicMaterial({ 
      color, 
      transparent: true, 
      opacity: isHighlighted ? 0.8 : 0.4 
    });
  }, [color, isHighlighted]);

  useFrame((state, delta) => {
    // Animate particle along the line
    if (particleRef.current && startPos && endPos) {
      progress.current = (progress.current + delta * 0.3) % 1;
      const pos = new THREE.Vector3().lerpVectors(startPos, endPos, progress.current);
      particleRef.current.position.copy(pos);
    }

    // Update line geometry
    if (lineRef.current && startPos && endPos) {
      const positions = lineRef.current.geometry.attributes.position as THREE.BufferAttribute;
      positions.setXYZ(0, startPos.x, startPos.y, startPos.z);
      positions.setXYZ(1, endPos.x, endPos.y, endPos.z);
      positions.needsUpdate = true;
    }
  });

  return (
    <group>
      {/* Main connection line */}
      <primitive object={new THREE.Line(lineGeometry, lineMaterial)} ref={lineRef} />

      {/* Animated particle */}
      <mesh ref={particleRef}>
        <sphereGeometry args={[0.02, 8, 8]} />
        <meshBasicMaterial color={color} transparent opacity={0.8} />
      </mesh>
    </group>
  );
}

function KnowledgeGraphScene() {
  const groupRef = useRef<THREE.Group>(null);
  const [hoveredNode, setHoveredNode] = useState<string | null>(null);
  const nodePositions = useRef<NodeRefs>({});

  const nodes: NodeData[] = useMemo(() => [
    { id: '1', label: 'User Profile', type: 'user', basePosition: [0, 0, 0], connections: ['2', '3', '4'] },
    { id: '2', label: 'Thai Food', type: 'entity', basePosition: [1.3, 0.6, -0.4], connections: ['5'] },
    { id: '3', label: 'Peanut Allergy', type: 'alert', basePosition: [-1.1, 0.4, 0.4], connections: ['5'] },
    { id: '4', label: 'Lunch Meetings', type: 'pattern', basePosition: [0.4, -0.9, 0.3], connections: ['2'] },
    { id: '5', label: 'Diet Conflict', type: 'insight', basePosition: [0.1, 1.1, -0.2], connections: [] },
    { id: '6', label: 'Work Schedule', type: 'entity', basePosition: [-1.3, -0.7, -0.3], connections: ['1', '4'] },
    { id: '7', label: 'Team Sync', type: 'pattern', basePosition: [1.5, -0.4, 0.5], connections: ['1', '4'] },
  ], []);

  // Build all connections (bidirectional visualization)
  const connections = useMemo(() => {
    const conns: { sourceId: string; targetId: string; color: string }[] = [];
    const addedPairs = new Set<string>();

    nodes.forEach((node) => {
      node.connections.forEach((targetId) => {
        const pairKey = [node.id, targetId].sort().join('-');
        if (!addedPairs.has(pairKey)) {
          addedPairs.add(pairKey);
          conns.push({
            sourceId: node.id,
            targetId,
            color: nodeColors[node.type],
          });
        }
      });
    });

    return conns;
  }, [nodes]);

  const handlePositionUpdate = (id: string, position: THREE.Vector3) => {
    nodePositions.current[id] = position;
  };

  useFrame((state) => {
    if (groupRef.current) {
      groupRef.current.rotation.y = state.clock.elapsedTime * 0.08;
    }
  });

  return (
    <group ref={groupRef}>
      {/* Render connections first (behind nodes) */}
      {connections.map((conn, i) => {
        const startPos = nodePositions.current[conn.sourceId];
        const endPos = nodePositions.current[conn.targetId];
        const isHighlighted = hoveredNode === conn.sourceId || hoveredNode === conn.targetId;

        if (!startPos || !endPos) {
          // Initialize with base positions
          const sourceNode = nodes.find(n => n.id === conn.sourceId);
          const targetNode = nodes.find(n => n.id === conn.targetId);
          if (sourceNode && targetNode) {
            return (
              <AnimatedConnection
                key={i}
                startPos={new THREE.Vector3(...sourceNode.basePosition)}
                endPos={new THREE.Vector3(...targetNode.basePosition)}
                color={conn.color}
                isHighlighted={isHighlighted}
              />
            );
          }
          return null;
        }

        return (
          <AnimatedConnection
            key={i}
            startPos={startPos}
            endPos={endPos}
            color={conn.color}
            isHighlighted={isHighlighted}
          />
        );
      })}

      {/* Render nodes */}
      {nodes.map((node) => (
        <GraphNode
          key={node.id}
          node={node}
          isHovered={hoveredNode === node.id}
          onHover={() => setHoveredNode(node.id)}
          onUnhover={() => setHoveredNode(null)}
          onPositionUpdate={handlePositionUpdate}
        />
      ))}
    </group>
  );
}

export function KnowledgeGraph3D() {
  return (
    <div className="w-full h-[500px] relative">
      <Canvas camera={{ position: [0, 0, 4.5], fov: 50 }}>
        <ambientLight intensity={0.5} />
        <pointLight position={[10, 10, 10]} intensity={1} />
        <pointLight position={[-10, -10, -10]} intensity={0.5} color="#a855f7" />
        <pointLight position={[0, 5, 0]} intensity={0.3} color="#00d4ff" />
        <KnowledgeGraphScene />
      </Canvas>
      
      {/* Legend */}
      <div className="absolute bottom-4 left-4 flex flex-wrap gap-3 text-xs">
        {Object.entries(nodeColors).map(([type, color]) => (
          <div key={type} className="flex items-center gap-1.5">
            <div
              className="w-3 h-3 rounded-full"
              style={{ backgroundColor: color, boxShadow: `0 0 8px ${color}` }}
            />
            <span className="text-muted-foreground capitalize">{type}</span>
          </div>
        ))}
      </div>
    </div>
  );
}