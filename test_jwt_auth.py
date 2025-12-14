"""Test JWT authentication with the Reflective Memory Kernel."""
import jwt
import datetime
import requests

# Generate test tokens - must match JWT_SECRET in .env
SECRET = "iXtlYLILSaXZrlJLNZNYuYf7A6fYFrwnPMvRDW0D848="

def generate_token(user_id: str) -> str:
    """Generate a JWT token for testing."""
    payload = {
        "sub": user_id,
        "exp": datetime.datetime.utcnow() + datetime.timedelta(hours=24),
        "iat": datetime.datetime.utcnow(),
    }
    return jwt.encode(payload, SECRET, algorithm="HS256")

def test_anonymous_access():
    """Test access without JWT token (should use 'anonymous')."""
    print("=== Test 1: Anonymous Access (No Token) ===")
    resp = requests.post(
        "http://localhost:9090/api/chat",
        json={"message": "Hello, I am anonymous"},
        timeout=60
    )
    print(f"Status: {resp.status_code}")
    print(f"Response: {resp.json()['response'][:100]}...")
    print()

def test_authenticated_access(user_id: str):
    """Test access with JWT token."""
    print(f"=== Test 2: Authenticated Access (user: {user_id}) ===")
    token = generate_token(user_id)
    print(f"Token: {token[:50]}...")
    
    resp = requests.post(
        "http://localhost:9090/api/chat",
        json={"message": f"My name is {user_id} and I love pizza"},
        headers={"Authorization": f"Bearer {token}"},
        timeout=60
    )
    print(f"Status: {resp.status_code}")
    try:
        print(f"Response: {resp.json()['response'][:100]}...")
    except:
        print(f"Response: {resp.text[:100]}...")
    print()

def test_invalid_token():
    """Test access with invalid JWT token."""
    print("=== Test 3: Invalid Token ===")
    resp = requests.post(
        "http://localhost:9090/api/chat",
        json={"message": "This should fail"},
        headers={"Authorization": "Bearer invalid_token_here"},
        timeout=60
    )
    print(f"Status: {resp.status_code}")
    print(f"Response: {resp.text[:100]}...")
    print()

if __name__ == "__main__":
    test_anonymous_access()
    test_authenticated_access("alice")
    test_invalid_token()
    print("=== All tests complete! ===")
