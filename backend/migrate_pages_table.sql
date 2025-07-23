-- Migration for new fields in pages table
ALTER TABLE pages
  ADD COLUMN internal_links INT DEFAULT 0,
  ADD COLUMN external_links INT DEFAULT 0,
  ADD COLUMN broken_links INT DEFAULT 0,
  ADD COLUMN broken_list TEXT,
  ADD COLUMN status VARCHAR(32) DEFAULT 'queued';

-- To support upsert, ensure url is unique
ALTER TABLE pages ADD UNIQUE KEY unique_url (url);
