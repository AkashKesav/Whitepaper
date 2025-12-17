"""Quick JWT test with new secret."""
import jwt
import datetime
import requests

# Must match JWT_SECRET in .env
SECRET = "i39kvQgLepT20Xk4Chjmmwr+vRq3Sg8zeVsGhlB5fc4="

# Generate token for user 'bob'
payload = {
    "sub": "bob",
    "exp": datetime.datetime.utcnow() + datetime.timedelta(hours=1)
}
token = jwt.encode(payload, SECRET, algorithm="HS256")
print(f"Token: {token[:50]}...")

# Send authenticated request
resp = requests.post(
    "http://localhost:3000/api/chat",
    json={"message": "I am Bob and I love coding"},
    headers={"Authorization": f"Bearer {token}"},
    timeout=60
)
print(f"Status: {resp.status_code}")
if resp.status_code == 200:
    print(f"Response: {resp.json()['response'][:100]}")
else:
    print(f"Error: {resp.text}")
