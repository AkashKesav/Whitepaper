#!/usr/bin/env python3
"""Generate test dataset with proper departments for compression testing."""
import json
import random

DEPARTMENTS = [
    "Engineering", "Product", "Design", "Marketing", "Sales", 
    "HR", "Finance", "Operations", "Data Science", "Security"
]

ROLES = [
    "Engineer", "Senior Engineer", "Manager", "Director", "Analyst", 
    "Lead", "Specialist", "Coordinator"
]

SKILLS = [
    "Python", "Go", "Java", "JavaScript", "React", "AWS", "K8s", 
    "Docker", "SQL", "ML", "Rust", "TypeScript", "GraphQL"
]

FIRST_NAMES = ["Alice", "Bob", "Carol", "David", "Emma", "Frank", "Grace", "Henry", "Iris", "Jack"]
LAST_NAMES = ["Smith", "Johnson", "Williams", "Brown", "Davis", "Garcia", "Miller", "Wilson"]

def generate_record(i):
    dept = random.choice(DEPARTMENTS)
    role = random.choice(ROLES)
    first = random.choice(FIRST_NAMES)
    last = random.choice(LAST_NAMES)
    
    return {
        "id": f"emp-{i:05d}",
        "name": f"{first} {last}",
        "department": dept,
        "role": f"{role} - {dept}",
        "skills": random.sample(SKILLS, random.randint(2, 5)),
        "level": random.randint(1, 5),
        "location": random.choice(["NYC", "SF", "Remote", "Austin", "Seattle"]),
        "years_exp": random.randint(1, 20)
    }

def main():
    records = [generate_record(i) for i in range(200)]
    
    with open("testdata/test_200.jsonl", "w") as f:
        for r in records:
            f.write(json.dumps(r) + "\n")
    
    # Count departments
    dept_counts = {}
    for r in records:
        dept_counts[r["department"]] = dept_counts.get(r["department"], 0) + 1
    
    print(f"Generated 200 records to testdata/test_200.jsonl")
    print("Department distribution:")
    for dept, count in sorted(dept_counts.items(), key=lambda x: x[1], reverse=True):
        print(f"  {dept}: {count}")

if __name__ == "__main__":
    main()
