import requests
import time
import json

# Test extraction optimization
url = "http://localhost:3000/api/chat"

# Test cases
tests = [
    ("Hello!", "Chitchat - should skip extraction"),
    ("Thanks!", "Chitchat - should skip extraction"),
    ("My favorite animal is a penguin.", "Should extract 'Penguin' entity"),
    ("My sister Emma lives in Boston.", "Should extract 'Emma' and 'Boston'"),
]

print()
print("╔══════════════════════════════════════════════════════════╗")
print("║           EXTRACTION OPTIMIZATION TEST                   ║")
print("╠══════════════════════════════════════════════════════════╣")
print("║  Optimizations:                                          ║")
print("║  ✓ Chitchat Filter (regex skip)                          ║")
print("║  ✓ Richer Prompt (better quality)                        ║")
print("║  ✓ Configurable Model (EXTRACTION_MODEL env)             ║")
print("╚══════════════════════════════════════════════════════════╝")
print()

for i, (message, expected) in enumerate(tests, 1):
    print(f"┌─ Test {i}/{len(tests)} ─────────────────────────────────────────────")
    print(f"│ Message:  \"{message}\"")
    print(f"│ Expected: {expected}")
    
    start = time.time()
    try:
        response = requests.post(url, json={"message": message}, timeout=90)
        elapsed = time.time() - start
        
        status = "✅ OK" if response.status_code == 200 else f"❌ {response.status_code}"
        print(f"│ Status:   {status}")
        print(f"│ Time:     {elapsed:.2f}s")
        
        if response.status_code == 200:
            data = response.json()
            resp_text = data.get('response', '')[:60].replace('\n', ' ')
            print(f"│ Response: \"{resp_text}...\"")
    except Exception as e:
        print(f"│ Error:    ❌ {e}")
    
    print(f"└──────────────────────────────────────────────────────────")
    print()

print("═══════════════════════════════════════════════════════════")
print("                    TEST COMPLETE                          ")
print("═══════════════════════════════════════════════════════════")
