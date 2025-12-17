import requests
import json

url = "http://localhost:8000/extract"
payload = {
    "user_query": "My favorite metal is Platinum.",
    "ai_response": "Noted."
}
headers = {"Content-Type": "application/json"}

try:
    response = requests.post(url, json=payload)
    print(f"Status: {response.status_code}")
    print(response.text)
except Exception as e:
    print(e)
