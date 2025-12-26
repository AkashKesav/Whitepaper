#!/usr/bin/env python3
"""
Generate a large synthetic dataset for migration testing.
Creates ~1GB of realistic employee, company, and project data.
Run: python3 generate_large_dataset.py
Output: testdata/large_dataset.jsonl
"""

import json
import random
import string
from datetime import datetime, timedelta
import sys

# Target size in bytes (~1GB for quick test, can scale to 3GB)
TARGET_SIZE_GB = 1.0
TARGET_SIZE_BYTES = int(TARGET_SIZE_GB * 1024 * 1024 * 1024)

# Sample data pools
FIRST_NAMES = [
    "James", "Mary", "John", "Patricia", "Robert", "Jennifer", "Michael", "Linda",
    "William", "Elizabeth", "David", "Barbara", "Richard", "Susan", "Joseph", "Jessica",
    "Thomas", "Sarah", "Charles", "Karen", "Christopher", "Lisa", "Daniel", "Nancy",
    "Matthew", "Betty", "Anthony", "Margaret", "Mark", "Sandra", "Donald", "Ashley",
    "Steven", "Kimberly", "Paul", "Emily", "Andrew", "Donna", "Joshua", "Michelle",
    "Kenneth", "Dorothy", "Kevin", "Carol", "Brian", "Amanda", "George", "Melissa",
    "Timothy", "Deborah", "Ronald", "Stephanie", "Edward", "Rebecca", "Jason", "Sharon"
]

LAST_NAMES = [
    "Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis",
    "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson",
    "Thomas", "Taylor", "Moore", "Jackson", "Martin", "Lee", "Perez", "Thompson",
    "White", "Harris", "Sanchez", "Clark", "Ramirez", "Lewis", "Robinson", "Walker",
    "Young", "Allen", "King", "Wright", "Scott", "Torres", "Nguyen", "Hill", "Flores"
]

COMPANIES = [
    "TechCorp", "DataSystems", "CloudNine", "InnovateTech", "FutureWorks",
    "QuantumLabs", "NexGen", "Synergy", "Apex", "Velocity", "Pinnacle",
    "Horizon", "Catalyst", "Elevate", "Pioneer", "Vanguard", "Titan", "Atlas"
]

DEPARTMENTS = [
    "Engineering", "Product", "Design", "Marketing", "Sales", "HR", "Finance",
    "Operations", "Legal", "Customer Success", "Data Science", "Security",
    "Infrastructure", "DevOps", "QA", "Research", "Business Development"
]

ROLES = [
    "Software Engineer", "Senior Software Engineer", "Staff Engineer",
    "Principal Engineer", "Engineering Manager", "Director of Engineering",
    "VP of Engineering", "CTO", "Product Manager", "Senior PM", "Designer",
    "UX Researcher", "Data Scientist", "ML Engineer", "DevOps Engineer",
    "Site Reliability Engineer", "Security Engineer", "QA Engineer", "Analyst"
]

SKILLS = [
    "Python", "Go", "Java", "JavaScript", "TypeScript", "Rust", "C++", "Ruby",
    "Kubernetes", "Docker", "AWS", "GCP", "Azure", "Terraform", "Ansible",
    "PostgreSQL", "MySQL", "MongoDB", "Redis", "Kafka", "React", "Vue", "Angular",
    "Node.js", "FastAPI", "Django", "Flask", "gRPC", "GraphQL", "REST",
    "Machine Learning", "Deep Learning", "NLP", "Computer Vision", "MLOps",
    "CI/CD", "Git", "Linux", "Networking", "Security", "Agile", "Scrum"
]

PROJECTS = [
    "Platform Modernization", "Cloud Migration", "Data Pipeline", "ML Platform",
    "Customer Portal", "Mobile App", "API Gateway", "Analytics Dashboard",
    "Identity Service", "Payment System", "Search Engine", "Recommendation Engine",
    "Notification Service", "Logging Platform", "Monitoring System", "CI/CD Pipeline"
]


def random_email(first, last, company):
    domain = company.lower().replace(" ", "") + ".com"
    return f"{first.lower()}.{last.lower()}@{domain}"


def random_date(start_year=2015, end_year=2024):
    start = datetime(start_year, 1, 1)
    end = datetime(end_year, 12, 31)
    delta = end - start
    random_days = random.randint(0, delta.days)
    return (start + timedelta(days=random_days)).isoformat()


def random_skills(n=5):
    return random.sample(SKILLS, min(n, len(SKILLS)))


def random_text(words=50):
    """Generate random text for descriptions."""
    lorem = [
        "innovative", "strategic", "collaborative", "results-driven", "dynamic",
        "experienced", "passionate", "dedicated", "technical", "analytical",
        "communication", "leadership", "problem-solving", "teamwork", "creative",
        "developing", "implementing", "managing", "optimizing", "scaling",
        "architecture", "solutions", "systems", "platforms", "applications"
    ]
    return " ".join(random.choices(lorem, k=words))


def generate_employee(emp_id):
    first = random.choice(FIRST_NAMES)
    last = random.choice(LAST_NAMES)
    company = random.choice(COMPANIES)
    
    return {
        "id": f"emp-{emp_id:08d}",
        "type": "employee",
        "first_name": first,
        "last_name": last,
        "full_name": f"{first} {last}",
        "email": random_email(first, last, company),
        "company": company,
        "department": random.choice(DEPARTMENTS),
        "role": random.choice(ROLES),
        "level": random.randint(1, 10),
        "salary": random.randint(50000, 300000),
        "hire_date": random_date(),
        "skills": random_skills(random.randint(3, 8)),
        "projects": random.sample(PROJECTS, random.randint(1, 4)),
        "manager_id": f"emp-{random.randint(1, max(1, emp_id-1)):08d}" if emp_id > 1 else None,
        "direct_reports": random.randint(0, 10),
        "performance_rating": round(random.uniform(2.5, 5.0), 1),
        "bio": random_text(random.randint(20, 100)),
        "is_active": random.random() > 0.1,
        "location": random.choice(["NYC", "SF", "LA", "Seattle", "Austin", "Denver", "Chicago", "Boston", "Remote"]),
        "timezone": random.choice(["US/Eastern", "US/Pacific", "US/Central", "US/Mountain", "UTC"]),
        "slack_handle": f"@{first.lower()}.{last.lower()}",
        "github_username": f"{first.lower()}{last.lower()}{random.randint(1, 999)}",
        "linkedin_url": f"https://linkedin.com/in/{first.lower()}-{last.lower()}-{random.randint(10000, 99999)}",
        "certifications": random.sample(["AWS Certified", "K8s Admin", "PMP", "Scrum Master", "GCP Pro"], random.randint(0, 3)),
        "education": random.choice(["BS Computer Science", "MS Computer Science", "PhD", "Bootcamp", "Self-taught"]),
        "years_experience": random.randint(1, 25),
        "last_promotion": random_date(2020, 2024),
        "notes": random_text(random.randint(50, 200)),
    }


def generate_company(company_id, name):
    return {
        "id": f"company-{company_id:04d}",
        "type": "company",
        "name": name,
        "industry": random.choice(["Technology", "Finance", "Healthcare", "Retail", "Manufacturing"]),
        "founded": random.randint(1990, 2020),
        "headquarters": random.choice(["San Francisco, CA", "New York, NY", "Seattle, WA", "Austin, TX"]),
        "employee_count": random.randint(100, 10000),
        "revenue": random.randint(1000000, 1000000000),
        "description": random_text(100),
        "website": f"https://www.{name.lower()}.com",
        "stock_symbol": name[:4].upper() if random.random() > 0.5 else None,
    }


def generate_project(proj_id, name):
    return {
        "id": f"project-{proj_id:04d}",
        "type": "project",
        "name": name,
        "status": random.choice(["Active", "Completed", "On Hold", "Planning"]),
        "start_date": random_date(2022, 2024),
        "end_date": random_date(2024, 2025) if random.random() > 0.3 else None,
        "budget": random.randint(100000, 5000000),
        "team_size": random.randint(3, 30),
        "tech_stack": random_skills(random.randint(4, 10)),
        "description": random_text(150),
        "priority": random.choice(["Critical", "High", "Medium", "Low"]),
        "completion_percent": random.randint(0, 100),
    }


def main():
    output_file = "testdata/large_dataset.jsonl"
    total_bytes = 0
    record_count = 0
    
    print(f"Generating {TARGET_SIZE_GB} GB dataset...")
    print(f"Target: {TARGET_SIZE_BYTES:,} bytes")
    
    with open(output_file, "w") as f:
        # Generate companies first
        for i, name in enumerate(COMPANIES):
            record = generate_company(i + 1, name)
            line = json.dumps(record) + "\n"
            f.write(line)
            total_bytes += len(line.encode())
            record_count += 1
        
        # Generate projects
        for i, name in enumerate(PROJECTS):
            record = generate_project(i + 1, name)
            line = json.dumps(record) + "\n"
            f.write(line)
            total_bytes += len(line.encode())
            record_count += 1
        
        # Generate employees until we hit target size
        emp_id = 1
        last_progress = 0
        while total_bytes < TARGET_SIZE_BYTES:
            record = generate_employee(emp_id)
            line = json.dumps(record) + "\n"
            f.write(line)
            total_bytes += len(line.encode())
            record_count += 1
            emp_id += 1
            
            # Progress update every 10%
            progress = int((total_bytes / TARGET_SIZE_BYTES) * 100)
            if progress >= last_progress + 10:
                print(f"  {progress}% complete ({record_count:,} records, {total_bytes / (1024*1024):.1f} MB)")
                last_progress = progress
    
    print(f"\nGenerated {output_file}")
    print(f"  Total records: {record_count:,}")
    print(f"  Total size: {total_bytes / (1024*1024):.1f} MB ({total_bytes / (1024*1024*1024):.2f} GB)")
    print(f"  Avg record size: {total_bytes / record_count:.0f} bytes")


if __name__ == "__main__":
    main()
