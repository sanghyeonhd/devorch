-- Step 14: router_policy를 scope-aware로 확장

ALTER TABLE router_policy ADD COLUMN scope_type TEXT NOT NULL DEFAULT 'global';
ALTER TABLE router_policy ADD COLUMN scope_id   TEXT NOT NULL DEFAULT '*';

-- 기존 UNIQUE 제약이 scope를 고려하지 못하므로, 새 테이블을 만드는 방식이 안전하지만
-- 여기서는 "단순화를 위해" 기존 인덱스를 새 인덱스로 대체한다.
-- SQLite에서 제약 수정은 까다로우니, 실제 운영에서는 "테이블 재작성"을 권장.

DROP INDEX IF EXISTS idx_router_policy_key;

CREATE INDEX IF NOT EXISTS idx_router_policy_scope_key
  ON router_policy(scope_type, scope_id, os, arch, scenario);
