import requests
import json

url = "http://localhost:3000/api/chat"
payload = {"message": "What is my favorite metal?"}

print(f"Testing {url}...")
try:
    response = requests.post(url, json=payload, timeout=60)
    print(f"Status: {response.status_code}")
    data = response.json()
    print(f"\nResponse:\n{json.dumps(data, indent=2)}")
except Exception as e:
    print(f"Error: {e}")
