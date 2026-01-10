#!/usr/bin/env python3
import httpx
import uuid
import time

BASE_URL = "http://localhost:9090"
AI_URL = "http://localhost:8000"

suffix = str(uuid.uuid4())[:8]
client = httpx.Client(timeout=60)

print("="*50)
print("MEMORY & DOCUMENT INGESTION TESTS")
print("="*50)

# Register and login
admin = client.post(f"{BASE_URL}/api/register", json={"username": f"adm_{suffix}", "password": "pass123"}).json()
print(f"Admin token obtained: {bool(admin.get('token'))}")

h = {"Authorization": f"Bearer {admin.get('token')}"}

# Test memory storage
r = client.post(f"{BASE_URL}/api/chat", headers=h, json={"message": "My favorite color is blue"})
print(f"Memory store status: {r.status_code}")

time.sleep(2)

# Test memory recall
r = client.post(f"{BASE_URL}/api/chat", headers=h, json={"message": "What is my favorite color?"})
print(f"Memory recall status: {r.status_code}")
if r.status_code == 200:
    resp = r.json().get("response", "")
    print(f"Contains 'blue': {'blue' in resp.lower()}")
    print(f"Response preview: {resp[:300]}")

# Graph stats
r = client.get(f"{BASE_URL}/api/stats", headers=h)
print(f"\nStats status: {r.status_code}")
if r.status_code == 200:
    print(f"Stats: {r.text[:500]}")

# AI Services health
r = client.get(f"{AI_URL}/health")
print(f"\nAI Services health: {r.status_code}")

# Document ingestion
print("\n--- Document Ingestion ---")
r = client.post(f"{AI_URL}/ingest", json={
    "text": "Meeting with Alice Johnson (CEO) about Q4 project deadline on Friday. Bob Smith will present technical roadmap.",
    "document_type": "meeting_notes"
})
print(f"Ingest status: {r.status_code}")
if r.status_code == 200:
    data = r.json()
    print(f"Response keys: {list(data.keys())}")
    if "entities" in data:
        print(f"Entities extracted: {len(data['entities'])}")
        for e in data.get("entities", [])[:5]:
            print(f"  - {e}")
    if "chunks" in data:
        print(f"Chunks created: {len(data['chunks'])}")
else:
    print(f"Error: {r.text[:300]}")

client.close()
print("\n" + "="*50)
