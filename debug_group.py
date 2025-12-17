import httpx
import json

# First get user UID
user_query = """
{
  user(func: eq(name, "manual_admin")) {
    uid
    name
  }
}
"""
r = httpx.post('http://localhost:8180/query', content=user_query, headers={'Content-Type': 'application/dql'})
user_uid = r.json()['data']['user'][0]['uid']
print(f"User UID: {user_uid}")

# Now test the exact IsWorkspaceMember query
ns = "group_a2f5af53-04d3-4c02-8e0a-4b7ae18b8166"
query = f"""
{{
  g(func: eq(namespace, "{ns}")) @filter(type(Group) AND (uid_in(group_has_admin, {user_uid}) OR uid_in(group_has_member, {user_uid}))) {{
    uid
    name
  }}
}}
"""
print(f"Query: {query}")
r = httpx.post('http://localhost:8180/query', content=query, headers={'Content-Type': 'application/dql'})
print(json.dumps(r.json(), indent=2))
