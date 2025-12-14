import requests
import websocket
import json
import time
import threading

BASE_URL = "http://localhost:9090/api"
WS_URL = "ws://localhost:9090/ws/chat"

username = "chronos_traveler"
password = "password123"
token = ""

def register_and_login():
    global token
    print(f"Registering user '{username}'...")
    try:
        requests.post(f"{BASE_URL}/register", json={"username": username, "password": password})
    except:
        pass # Might exist
        
    print(f"Logging in user '{username}'...")
    resp = requests.post(f"{BASE_URL}/login", json={"username": username, "password": password})
    if resp.status_code == 200:
        token = resp.json().get("token")
        print(f"Login successful! Token: {token[:10]}...")
    else:
        print(f"Login failed: {resp.text}")
        exit(1)

def seed_knowledge():
    print("Seeding knowledge...")
    # Just chat so it gets ingested
    headers = {"Authorization": f"Bearer {token}"}
    chat_resp = requests.post(f"{BASE_URL}/chat", json={
        "message": "Akash is the creator of the Time Machine protocol.",
        "user_id": username
    }, headers=headers)
    print("Seed response:", chat_resp.json().get("response"))
    time.sleep(2) # Allow ingestion

def ws_client():
    headers = [f"Authorization: Bearer {token}"]
    ws = websocket.WebSocket()
    ws.connect(WS_URL, header=headers)
    print("WebSocket Connected!")
    
    # 1. Simulate Typing
    partial_query = "Who is the creator of the Time"
    print(f"Sending TYPING event: '{partial_query}'")

    
    # Actually, server.go expects:
    # type WSMessage struct { Type string; Payload json.RawMessage }
    # Unmarshal(msg.Payload, &payload)
    # json.RawMessage IS bytes of the inner JSON. 
    # So we send: {"type": "typing", "payload": { "message": "..." }}
    
    valid_typing_msg = json.dumps({
        "type": "typing",
        "payload": {
            "message": partial_query,
            "context_type": "user"
        }
    })
    ws.send(valid_typing_msg)
    
    # Speculation happens on server...
    time.sleep(0.5) # Wait 500ms
    
    # 2. Send Actual Chat
    full_query = "Who is the creator of the Time Machine protocol?"
    print(f"Sending CHAT event: '{full_query}'")
    
    start_time = time.time()
    valid_chat_msg = json.dumps({
        "type": "chat",
        "payload": {
            "message": full_query,
            "context_type": "user"
        }
    })
    ws.send(valid_chat_msg)
    
    # 3. Read Response
    resp = ws.recv()
    end_time = time.time()
    
    print(f"Response received in {(end_time - start_time)*1000:.2f}ms")
    print("Response payload:", resp)
    
    ws.close()

if __name__ == "__main__":
    if requests.get(f"http://localhost:9090/health").status_code != 200:
        print("Server not ready")
        exit(1)
        
    register_and_login()
    seed_knowledge()
    print("Waiting for ingestion...")
    time.sleep(3)
    ws_client()
