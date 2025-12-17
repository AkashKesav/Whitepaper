import requests
import json

# Test embedding endpoint
url = "http://localhost:8000/embed"
payload = {"text": "What is my favorite metal?"}

print(f"Testing {url}...")
try:
    response = requests.post(url, json=payload, timeout=60)
    print(f"Status: {response.status_code}")
    data = response.json()
    if "embedding" in data:
        print(f"Embedding length: {len(data['embedding'])}")
        print(f"First 5 values: {data['embedding'][:5]}")
    else:
        print(f"Response: {data}")
except Exception as e:
    print(f"Error: {e}")
