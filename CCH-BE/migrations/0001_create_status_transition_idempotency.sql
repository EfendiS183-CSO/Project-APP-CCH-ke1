CREATE TABLE IF NOT EXISTS cch_be_status_transition_idempotency (
    idempotency_key VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    complaint_id VARCHAR(25) CHARACTER SET latin1 COLLATE latin1_swedish_ci NOT NULL,
    history_id VARCHAR(25) CHARACTER SET latin1 COLLATE latin1_swedish_ci NOT NULL,
    status_code CHAR(3) CHARACTER SET latin1 COLLATE latin1_swedish_ci NOT NULL,
    applied_at DATETIME(6) NOT NULL,
    PRIMARY KEY (idempotency_key),
    KEY idx_status_transition_complaint (complaint_id, applied_at)
) ENGINE=InnoDB DEFAULT CHARSET=ascii;