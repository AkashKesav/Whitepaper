<<<<<<< HEAD
"""
Extraction SLM - Extracts structured entities from conversations.
Optimized for speed with chitchat filtering and simplified prompts.
"""
import re
import os
from typing import Optional

from llm_router import LLMRouter


# Patterns for messages that don't need extraction
CHITCHAT_PATTERNS = [
    r'^(hi|hello|hey|yo|sup)[\s!.?]*$',
    r'^(bye|goodbye|see you|later|cya)[\s!.?]*$',
    r'^(thanks|thank you|thx|ty)[\s!.?]*$',
    r'^(ok|okay|sure|yes|no|yep|nope)[\s!.?]*$',
    r'^(good|great|nice|cool|awesome)[\s!.?]*$',
    r'^(how are you|what\'s up|how\'s it going)[\s!.?]*$',
    r'^(lol|haha|hehe|xd)[\s!.?]*$',
    r'^[\s.!?]+$',  # Just punctuation/whitespace
]

# Compile patterns for speed
CHITCHAT_REGEX = [re.compile(p, re.IGNORECASE) for p in CHITCHAT_PATTERNS]


def is_chitchat(text: str) -> bool:
    """Check if message is simple chitchat that doesn't need extraction."""
    text = text.strip()
    if len(text) < 3:
        return True
    for pattern in CHITCHAT_REGEX:
        if pattern.match(text):
            return True
    return False


class ExtractionSLM:
    """Lightweight extraction model for entity and relationship extraction.
    
    Optimizations:
    - Chitchat filter: Skip extraction for simple messages
    - Simplified prompt: Faster processing with few-shot format
    - Configurable model: Use EXTRACTION_MODEL env var for faster models
    """

    def __init__(self, router: LLMRouter):
        self.router = router
        self.provider = "nvidia"
        # Use meta/llama-3.1-70b-instruct for high quality extraction via NVIDIA NIM
        self.model = os.getenv("EXTRACTION_MODEL", "meta/llama-3.1-70b-instruct")

    async def extract(
        self,
        user_query: str,
        ai_response: str,
        context: Optional[str] = None,
    ) -> list:
        """Extract entities and relationships from a conversation turn."""
        
        # OPTIMIZATION 1: Skip chitchat messages (big time saver)
        if is_chitchat(user_query):
            print(f"DEBUG: Skipping chitchat: '{user_query[:30]}'", flush=True)
            return []
        
        # Improved prompt with concrete examples for better extraction
        prompt = f"""Extract entities from this conversation. Return a JSON array.

EXAMPLES:
Conversation:
User: "My favorite dessert is gulab jamun"
AI: "That sounds delicious."
Output: [{{"name": "Gulab Jamun", "type": "Preference", "description": "User's favorite dessert", "tags": ["food", "dessert", "favorite"]}}]

Conversation:
User: "My sister Emma lives in Boston"
AI: "I've noted that about Emma."
Output: [{{"name": "Emma", "type": "Entity", "description": "User's sister", "tags": ["family", "sister"]}}, {{"name": "Boston", "type": "Entity", "description": "Where Emma lives", "tags": ["city", "location"]}}]

Conversation:
User: "I like hiking"
AI: "Hiking is great exercise."
Output: [{{"name": "Hiking", "type": "Preference", "description": "Activity user enjoys", "tags": ["hobby", "activity", "outdoors"]}}]

Conversation:
User: "The weather is nice today"
AI: "Yes it is."
Output: []

NOW EXTRACT FROM:
Conversation:
User: "{user_query}"
AI: "{ai_response}"

Output JSON array (empty [] if nothing to extract):"""

        result = await self.router.extract_json(prompt, provider=self.provider, model=self.model)
        
        if isinstance(result, list):
            print(f"DEBUG: Extracted {len(result)} entities", flush=True)
            for e in result:
                print(f"DEBUG: Entity: {e.get('name')} - {e.get('description')}", flush=True)
            return result
        return []

=======
"""
Extraction SLM - Extracts structured entities from conversations.
Optimized for speed with chitchat filtering and simplified prompts.
"""
import re
import os
from typing import Optional

from llm_router import LLMRouter


# Patterns for messages that don't need extraction
CHITCHAT_PATTERNS = [
    r'^(hi|hello|hey|yo|sup)[\s!.?]*$',
    r'^(bye|goodbye|see you|later|cya)[\s!.?]*$',
    r'^(thanks|thank you|thx|ty)[\s!.?]*$',
    r'^(ok|okay|sure|yes|no|yep|nope)[\s!.?]*$',
    r'^(good|great|nice|cool|awesome)[\s!.?]*$',
    r'^(how are you|what\'s up|how\'s it going)[\s!.?]*$',
    r'^(lol|haha|hehe|xd)[\s!.?]*$',
    r'^[\s.!?]+$',  # Just punctuation/whitespace
]

# Compile patterns for speed
CHITCHAT_REGEX = [re.compile(p, re.IGNORECASE) for p in CHITCHAT_PATTERNS]


def is_chitchat(text: str) -> bool:
    """Check if message is simple chitchat that doesn't need extraction."""
    text = text.strip()
    if len(text) < 3:
        return True
    for pattern in CHITCHAT_REGEX:
        if pattern.match(text):
            return True
    return False


class ExtractionSLM:
    """Lightweight extraction model for entity and relationship extraction.
    
    Optimizations:
    - Chitchat filter: Skip extraction for simple messages
    - Simplified prompt: Faster processing with few-shot format
    - Configurable model: Use EXTRACTION_MODEL env var for faster models
    """

    def __init__(self, router: LLMRouter):
        self.router = router
        self.provider = "nvidia"
        # Use deepseek-ai/deepseek-v3.2 for high quality extraction via NVIDIA NIM
        self.model = os.getenv("EXTRACTION_MODEL", "deepseek-ai/deepseek-v3.2")

    async def extract(
        self,
        user_query: str,
        ai_response: str,
        context: Optional[str] = None,
    ) -> list:
        """Extract entities and relationships from a conversation turn."""
        
        # OPTIMIZATION 1: Skip chitchat messages (big time saver)
        if is_chitchat(user_query):
            print(f"DEBUG: Skipping chitchat: '{user_query[:30]}'", flush=True)
            return []
        
        # Improved prompt with concrete examples for better extraction
        prompt = f"""Extract entities from this conversation. Return a JSON array.

EXAMPLES:
Conversation:
User: "My favorite dessert is gulab jamun"
AI: "That sounds delicious."
Output: [{{"name": "Gulab Jamun", "type": "Preference", "description": "User's favorite dessert", "tags": ["food", "dessert", "favorite"]}}]

Conversation:
User: "My sister Emma lives in Boston"
AI: "I've noted that about Emma."
Output: [{{"name": "Emma", "type": "Entity", "description": "User's sister", "tags": ["family", "sister"]}}, {{"name": "Boston", "type": "Entity", "description": "Where Emma lives", "tags": ["city", "location"]}}]

Conversation:
User: "I like hiking"
AI: "Hiking is great exercise."
Output: [{{"name": "Hiking", "type": "Preference", "description": "Activity user enjoys", "tags": ["hobby", "activity", "outdoors"]}}]

Conversation:
User: "The weather is nice today"
AI: "Yes it is."
Output: []

NOW EXTRACT FROM:
Conversation:
User: "{user_query}"
AI: "{ai_response}"

Output JSON array (empty [] if nothing to extract):"""

        result = await self.router.extract_json(prompt, provider=self.provider, model=self.model)
        
        if isinstance(result, list):
            print(f"DEBUG: Extracted {len(result)} entities", flush=True)
            for e in result:
                print(f"DEBUG: Entity: {e.get('name')} - {e.get('description')}", flush=True)
            return result
        return []

>>>>>>> 5f37bd4 (Major update: API timeout fixes, Vector-Native ingestion, Frontend integration)
