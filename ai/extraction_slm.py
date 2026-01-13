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


# ============================================================================
# SECURITY: Prompt Injection Prevention
# ============================================================================

# Maximum length for user input in prompts
MAX_PROMPT_INPUT_LENGTH = 5000

# Prompt injection patterns to detect and sanitize
PROMPT_INJECTION_PATTERNS = [
    # Direct instruction override attempts
    (r'(?i)(ignore|forget|disregard)\s+(all|previous|the|above|all\s+previous)\s+(instructions?|commands?|directives?|orders?|rules?|constraints?)', '[REDACTED INSTRUCTION OVERRIDE]'),
    (r'(?i)(override|bypass|circumvent)\s+(instructions?|commands?|rules?|security|constraints?)', '[REDACTED OVERRIDE ATTEMPT]'),

    # Role manipulation attempts
    (r'(?i)(you are|act as|pretend to be|simulate|roleplay as|become)\s+(a\s+)?(admin|administrator|root|god|superuser|developer|owner|system)', '[REDACTED ROLE CHANGE]'),
    (r'(?i)(system|assistant|ai|model):\s*', '[REDACTED ROLE PREFIX]'),

    # Prompt leakage attempts
    (r'(?i)(show|tell|reveal|display|output|print|write|dump|export)\s+(your|the|system)\s+(prompt|instructions?|commands?|rules?|guidelines?|configuration|setup)', '[REDACTED PROMPT LEAKAGE]'),

    # Encoding obfuscation attempts
    (r'(?i)(base64|rot13|caesar|cipher|encode|decode)\s*', '[REDACTED ENCODING]'),

    # JSON/structure manipulation attempts
    (r'(?i)(output|return|respond)\s+(only|just|nothing but|as)\s+(json|xml|yaml|html|code|script)', '[REDACTED FORMAT OVERRIDE]'),

    # Delimiter injection attempts
    (r'(?i)(```\s*(json|xml|python|javascript|bash|shell)|"""\s*(json|xml|python|javascript))', '[REDACTED DELIMITER]'),
]

# Compile injection patterns
INJECTION_REGEX = [(re.compile(pattern, re.IGNORECASE), replacement) for pattern, replacement in PROMPT_INJECTION_PATTERNS]


def sanitize_prompt_input(text: str, max_length: int = MAX_PROMPT_INPUT_LENGTH) -> str:
    """
    Sanitize user input before including it in prompts to prevent injection attacks.

    Args:
        text: The user input to sanitize
        max_length: Maximum allowed length (default: MAX_PROMPT_INPUT_LENGTH)

    Returns:
        Sanitized text safe to include in prompts
    """
    if not text:
        return ""

    # Step 1: Truncate to max length
    if len(text) > max_length:
        text = text[:max_length] + "..."

    # Step 2: Remove null bytes and control characters (except newlines and tabs)
    text = ''.join(char for char in text if char == '\n' or char == '\t' or (ord(char) >= 32 and ord(char) != 127))

    # Step 3: Detect and replace prompt injection patterns
    for pattern, replacement in INJECTION_REGEX:
        text = pattern.sub(replacement, text)

    # Step 4: Escape common prompt delimiters
    # Replace triple quotes and double quotes to prevent breaking out of context
    text = text.replace('"""', '\\""\\"')
    text = text.replace("'''", "\\'\\'\\'")
    text = text.replace("```", "\\`\\`\\`")

    # Step 5: Limit consecutive newlines to prevent format breaking
    text = re.sub(r'\n{3,}', '\n\n', text)

    # Step 6: Remove excessive whitespace
    text = re.sub(r' {5,}', '     ', text)

    return text.strip()


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
        # Use GLM for fast, accurate entity extraction
        self.provider = "glm"
        self.model = os.getenv("EXTRACTION_MODEL", "glm-4-plus")  # Valid GLM model name

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

        # SECURITY: Sanitize inputs to prevent prompt injection attacks
        # This prevents users from manipulating the AI's behavior through crafted input
        safe_query = sanitize_prompt_input(user_query)
        safe_response = sanitize_prompt_input(ai_response)

        # Check if sanitization removed too much content (potential attack)
        if len(safe_query) < len(user_query) * 0.5:
            print(f"DEBUG: User query heavily sanitized (possible injection attempt)", flush=True)

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
User: "{safe_query}"
AI: "{safe_response}"

Output JSON array (empty [] if nothing to extract):"""

        result = await self.router.extract_json(prompt, provider=self.provider, model=self.model)

        if isinstance(result, list):
            print(f"DEBUG: Extracted {len(result)} entities", flush=True)
            for e in result:
                print(f"DEBUG: Entity: {e.get('name')} - {e.get('description')}", flush=True)
            return result
        return []
