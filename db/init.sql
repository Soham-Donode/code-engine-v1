CREATE TABLE IF NOT EXISTS submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    language VARCHAR(50) NOT NULL,
    code TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'queued', -- queued, running, completed, error, timeout
    stdout TEXT,
    stderr TEXT,
    execution_time_ms INT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_submissions_status ON submissions(status);