import requests
import json
import time

BASE_URL = "http://localhost:9090/api"
session = requests.Session()

def register_and_get_token():
    # 1. Register
    reg_payload = {"username": "quantum_user_docker", "password": "password123"}
    try:
        print(f"Attempting to register at {BASE_URL}/register")
        resp = session.post(f"{BASE_URL}/register", json=reg_payload)
        
        if resp.status_code == 409: # Already exists
            print("User already exists, logging in...")
            resp = session.post(f"{BASE_URL}/login", json=reg_payload)
        
        if resp.status_code != 200:
            print(f"Auth failed: {resp.status_code} {resp.text}")
            return None
            
        token = resp.json().get("token")
        print(f"Authenticated as quantum_user_docker. Token: {token[:10]}...")
        return token
    except Exception as e:
        print(f"Auth Error: {e}")
        return None

def chat(token):
    url = f"{BASE_URL}/chat"
    payload = {
        "user_id": "quantum_user_docker",
        "message": "Protocol Omega: Docker System Check."
    }
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {token}"
    }

    print(f"Sending chat request to {url}...")
    try:
        start = time.time()
        response = session.post(url, json=payload, headers=headers)
        latency = (time.time() - start) * 1000
        
        print(f"Status Code: {response.status_code}")
        print(f"Latency: {latency:.2f}ms")
        if response.status_code == 200:
            print("Response Body:")
            print(json.dumps(response.json(), indent=2))
        else:
            print(f"Error: {response.text}")
    except Exception as e:
        print(f"Request failed: {e}")

if __name__ == "__main__":
    # Check Health
    try:
        h = requests.get("http://localhost:9090/health")
        print(f"Health Check: {h.status_code} {h.text}")
    except Exception as e:
        print(f"Health Check Failed: {e}")
    
    token = register_and_get_token()
    if token:
        chat(token)
