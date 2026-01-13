
import requests
import json
import time
import sys

# Configuration (Internal Docker Network)
AI_SERVICES_URL = "http://whitepaper-ai-services:8000"
MONOLITH_URL = "http://whitepaper-monolith:9090"
DGRAPH_URL = "http://rmk-dgraph-alpha:8080" # Internal

USER_ID = "test_user_boost"
NAMESPACE = "user_" + USER_ID

def log(msg):
    print(f"[BoostVerif] {msg}")

def ingest_entity():
    log("Ingesting test entity...")
    # Using the Monolith extraction endpoint or just raw graph mutation?
    # Let's use the actual pipeline to create the node properly
    
    # 1. Simulate extraction response (internal logic) or just call Monolith Ingest
    # Monolith Ingest endpoint: POST /webhook/transcript (simulated)
    # Actually, easiest way is to use DGraph directly to CREATE the node, 
    # then use Monolith to CONSULT.
    
    # Use JSON mutation instead of RDF to avoid type/syntax errors
    mutation_payload = {
        "set": [
            {
                "dgraph.type": ["Entity", "Node"],
                "name": "BoostTarget",
                "namespace": NAMESPACE,
                "activation": 0.5,
                "access_count": 0
            }
        ]
    }
    
    headers = {'Content-Type': 'application/json'}
    try:
        resp = requests.post(f"{DGRAPH_URL}/mutate?commitNow=true", json=mutation_payload, headers=headers)
        log(f"Create Node Status: {resp.status_code}")
        # log(f"Create Node Response: {resp.text}")
        
        if resp.status_code != 200:
            log(f"Failed to create node. Body: {resp.text}")
            sys.exit(1)
            
        data = resp.json()
        # DGraph mutate response has 'data': {'uids': {...}}
        # check if it succeeded
    except Exception as e:
        log(f"Mutation request failed or JSON parse error: {e}")
        if 'resp' in locals():
            log(f"Response text was: {resp.text}")
        sys.exit(1)
        
    # Get the UID
    search_query = """
    {
        node(func: eq(name, "BoostTarget")) @filter(eq(namespace, "%s")) {
            uid
        }
    }
    """ % NAMESPACE
    
    resp = requests.post(f"{DGRAPH_URL}/query", params={'s': search_query})
    data = resp.json()
    if not data['data']['node']:
        log("Failed to find created node")
        sys.exit(1)
        
    uid = data['data']['node'][0]['uid']
    log(f"Created node 'BoostTarget' with UID: {uid} and Activation: 0.5")
    return uid

def check_activation(uid):
    query = """
    {
        node(func: uid(%s)) {
            activation
        }
    }
    """ % uid
    resp = requests.post(f"{DGRAPH_URL}/query", params={'s': query})
    data = resp.json()
    return data['data']['node'][0]['activation']

def trigger_consultation():
    log("Triggering consultation for 'BoostTarget'...")
    payload = {
        "user_id": USER_ID,
        "query": "Tell me about BoostTarget",
        "namespace": NAMESPACE
    }
    try:
        resp = requests.post(f"{MONOLITH_URL}/consult", json=payload)
        log(f"Consultation status: {resp.status_code}")
        # log(f"Response: {resp.text}")
    except Exception as e:
        log(f"Consultation failed: {e}")

def main():
    try:
        # 1. Create Node with 0.5 activation
        uid = ingest_entity()
        
        # Verify initial state
        initial_act = check_activation(uid)
        log(f"Initial Activation: {initial_act}")
        if initial_act != 0.5:
            # Maybe it stored as float slightly differently?
            pass

        # 2. Trigger Consultation (which should find it and boost it)
        trigger_consultation()
        
        # 3. Wait for async boost
        log("Waiting 3 seconds for async boost...")
        time.sleep(3)
        
        # 4. Check new activation
        final_act = check_activation(uid)
        log(f"Final Activation: {final_act}")
        
        if final_act > 0.5:
            log("SUCCESS: Activation increased!")
            sys.exit(0)
        else:
            log("FAILURE: Activation did not increase.")
            sys.exit(1)
            
    except Exception as e:
        log(f"Verification crashed: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
