PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (id TEXT PRIMARY KEY, email TEXT UNIQUE NOT NULL, display_name TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS goals (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id), title TEXT NOT NULL, active INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS tasks (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id), goal_id TEXT REFERENCES goals(id), title TEXT NOT NULL, duration_minutes INTEGER NOT NULL, completed INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS devices (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id), name TEXT NOT NULL, token_hash TEXT NOT NULL, last_seen_at TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS notes (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id), task_id TEXT REFERENCES tasks(id), transcript TEXT NOT NULL, context TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS completions (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id), task_id TEXT NOT NULL REFERENCES tasks(id), completed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, note_id TEXT REFERENCES notes(id));

INSERT OR IGNORE INTO users (id, email, display_name) VALUES ('demo-user', 'demo@dialed.local', 'You');
INSERT OR IGNORE INTO goals (id, user_id, title) VALUES ('today', 'demo-user', 'Make the next thing count.');
INSERT OR IGNORE INTO tasks (id, user_id, goal_id, title, duration_minutes, completed) VALUES
  ('website', 'demo-user', 'today', 'Refine the website', 42, 0),
  ('ab-test', 'demo-user', 'today', 'Create A/B test', 28, 0),
  ('interview', 'demo-user', 'today', 'Interview Rian', 19, 1);
