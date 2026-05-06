ALTER TABLE cs_sessions DROP CONSTRAINT IF EXISTS chk_cs_sessions_channel;

ALTER TABLE cs_sessions
ADD CONSTRAINT chk_cs_sessions_channel
CHECK (channel IN ('telegram','discord','wechat','widget','sub2api','newapi'));
