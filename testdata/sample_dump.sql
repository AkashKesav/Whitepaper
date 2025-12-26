-- Sample SQL dump for testing migration
-- PostgreSQL pg_dump format

INSERT INTO employees (id, name, email, role, team) VALUES 
(1, 'Alice Johnson', 'alice@acme.com', 'Senior Engineer', 'Platform'),
(2, 'Bob Smith', 'bob@acme.com', 'Engineering Manager', 'Platform'),
(3, 'Carol Williams', 'carol@acme.com', 'Product Manager', 'Product');

INSERT INTO teams (id, name, lead_id) VALUES
(1, 'Platform', 2),
(2, 'Product', 3);

INSERT INTO projects (id, name, description, team_id) VALUES
(1, 'Memory Kernel', 'Reflective AI memory system', 1),
(2, 'AI Services', 'LLM orchestration layer', 1);
