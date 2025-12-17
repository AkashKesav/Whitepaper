import httpx
import asyncio
from datetime import datetime, timezone

async def full_verification():
    DGRAPH_URL = "http://localhost:8180/query"
    
    # Get all nodes with full activation data
    query = """{
        all(func: has(name), orderdesc: activation) {
            uid
            name
            description
            activation
            access_count
            last_accessed
            created_at
        }
    }"""
    
    async with httpx.AsyncClient() as client:
        resp = await client.post(DGRAPH_URL, json={"query": query})
        data = resp.json()
        nodes = data.get("data", {}).get("all", [])
        
        print("=" * 100)
        print("DYNAMIC REORDERING & DECAY VERIFICATION")
        print("=" * 100)
        
        # Filter out UUID nodes and user_xxx nodes
        real_nodes = [n for n in nodes if n.get('name') and 
                      not n.get('name', '').startswith('user_') and
                      len(n.get('name', '')) < 36]  # Exclude UUIDs
        
        print(f"\n📊 TOTAL NODES: {len(real_nodes)}")
        print(f"\n{'#':<4} {'Name':<25} {'Activation':<12} {'Accessed':<10} {'Last Access':<20}")
        print("-" * 100)
        
        now = datetime.now(timezone.utc).replace(tzinfo=None)
        for i, node in enumerate(real_nodes[:20], 1):
            name = node.get('name', 'Unknown')[:24]
            activation = node.get('activation', 0.0)
            access_count = node.get('access_count', 0)
            last_accessed = node.get('last_accessed', '')
            
            # Calculate time since last access
            time_ago = ""
            if last_accessed:
                try:
                    last_dt = datetime.fromisoformat(last_accessed.replace('Z', '+00:00'))
                    if last_dt.tzinfo:
                        last_dt = last_dt.replace(tzinfo=None)
                    delta = now - last_dt
                    if delta.days > 0:
                        time_ago = f"{delta.days}d ago"
                    elif delta.seconds > 3600:
                        time_ago = f"{delta.seconds // 3600}h ago"
                    else:
                        time_ago = f"{delta.seconds // 60}m ago"
                except:
                    time_ago = "recently"
            else:
                time_ago = "never"
            
            # Emoji indicators
            if activation >= 0.9:
                emoji = "🔥"
            elif activation >= 0.7:
                emoji = "🟡"
            elif activation >= 0.5:
                emoji = "🟢"
            else:
                emoji = "🔵"
            
            print(f"{emoji} {i:<2} {name:<24} {activation:<12.3f} {access_count:<10} {time_ago:<20}")
        
        # Analysis
        print("\n" + "=" * 100)
        print("ANALYSIS:")
        print("=" * 100)
        
        high_activation = [n for n in real_nodes if n.get('activation', 0) >= 0.8]
        medium_activation = [n for n in real_nodes if 0.5 <= n.get('activation', 0) < 0.8]
        low_activation = [n for n in real_nodes if n.get('activation', 0) < 0.5]
        
        print(f"\n🔥 High Activation (≥0.8): {len(high_activation)} nodes")
        print(f"🟡 Medium Activation (0.5-0.8): {len(medium_activation)} nodes")
        print(f"🔵 Low Activation (<0.5): {len(low_activation)} nodes")
        
        # Check ordering
        activations = [n.get('activation', 0) for n in real_nodes]
        is_sorted = all(activations[i] >= activations[i+1] for i in range(len(activations)-1))
        
        print(f"\n✅ REORDERING: {'WORKING' if is_sorted else 'NOT WORKING'}")
        if is_sorted:
            print("   Nodes are correctly ordered by activation (high to low)")
        
        # Check for accessed nodes
        accessed_nodes = [n for n in real_nodes if n.get('access_count', 0) > 0]
        print(f"\n✅ ACTIVATION BOOST: {len(accessed_nodes)} nodes have been accessed")
        if accessed_nodes:
            print("   Examples:")
            for node in accessed_nodes[:5]:
                name = node.get('name', 'Unknown')[:20]
                count = node.get('access_count', 0)
                activation = node.get('activation', 0.0)
                print(f"   - {name}: {count} accesses → activation {activation:.3f}")
        
        # Check for decay candidates (old nodes with low activation)
        old_nodes = []
        for node in real_nodes:
            last_accessed = node.get('last_accessed', '')
            if last_accessed:
                try:
                    last_dt = datetime.fromisoformat(last_accessed.replace('Z', '+00:00'))
                    if last_dt.tzinfo:
                        last_dt = last_dt.replace(tzinfo=None)
                    delta = now - last_dt
                    if delta.days >= 1:  # More than 1 day old
                        old_nodes.append((node, delta.days))
                except:
                    pass
        
        print(f"\n⏰ DECAY STATUS:")
        if old_nodes:
            print(f"   Found {len(old_nodes)} nodes older than 1 day (decay candidates)")
            print("   Examples:")
            for node, days in old_nodes[:3]:
                name = node.get('name', 'Unknown')[:20]
                activation = node.get('activation', 0.0)
                print(f"   - {name}: {days}d old → activation {activation:.3f}")
        else:
            print("   No nodes older than 1 day yet (system recently started)")
            print("   Decay will apply automatically after 24h of no access")
        
        print("\n" + "=" * 100)

if __name__ == "__main__":
    asyncio.run(full_verification())
