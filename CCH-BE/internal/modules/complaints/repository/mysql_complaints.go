package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"cch.local/backend/internal/modules/complaints"
)

const complaintListSelectSQL = `SELECT
	ID_Pengaduan,
	Kantor_Asal,
	Kantor_Tujuan,
	Kantor_Tembusan,
	Semua_Tujuan,
	Nomor_Kiriman,
	Tanggal_Pengaduan,
	Ringkasan_Pengaduan,
	Status_Akhir,
	Tanggal_Status,
	Kantor_Status
FROM cchentri`

const complaintListOrderSQL = "ORDER BY Tanggal_Pengaduan DESC, ID_Pengaduan DESC LIMIT ? OFFSET ?"

const complaintHistoryQuerySQL = `SELECT
	h.ID_Pengaduan,
	h.ID_Histori,
	h.Status_Update,
	COALESCE(s.Deskripsi_Status, ''),
	h.Kantor_Asal_Update,
	h.Deskripsi_Update,
	h.Tanggal_Update,
	h.Petugas_Update,
	h.Durasi_Antar_Status
FROM cchentrimas AS h
INNER JOIN cchentri AS c ON c.ID_Pengaduan = h.ID_Pengaduan
LEFT JOIN cch_jenis_penanganan AS s ON s.ID_Status = h.Status_Update
WHERE h.ID_Pengaduan = ?
	AND c.Status_Akhir <> '109'
	AND (c.Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Semua_Tujuan, ''), ' ', '')) > 0)
ORDER BY h.Tanggal_Update ASC, h.ID_Histori ASC
LIMIT ? OFFSET ?`

const completedComplaintHistoryQuerySQL = `SELECT
	h.ID_Pengaduan,
	h.ID_Histori,
	h.Status_Update,
	COALESCE(s.Deskripsi_Status, ''),
	h.Kantor_Asal_Update,
	h.Deskripsi_Update,
	h.Tanggal_Update,
	h.Petugas_Update,
	h.Durasi_Antar_Status
FROM cchentrimas AS h
INNER JOIN cchentri AS c ON c.ID_Pengaduan = h.ID_Pengaduan
LEFT JOIN cch_jenis_penanganan AS s ON s.ID_Status = h.Status_Update
WHERE h.ID_Pengaduan = ?
	AND c.Status_Akhir = '109'
	AND (c.Kantor_Asal = ? OR c.Kantor_Tujuan = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Kantor_Tembusan, ''), ' ', '')) > 0)
ORDER BY h.Tanggal_Update ASC, h.ID_Histori ASC
LIMIT ? OFFSET ?`

const completedComplaintCountSQL = `SELECT COUNT(*)
FROM cchselesai
WHERE (Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(Semua_Tujuan, ''), ' ', '')) > 0)`

const openComplaintCountSQL = `SELECT COUNT(*)
FROM cchentri
WHERE Status_Akhir <> '109'
AND (Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(Semua_Tujuan, ''), ' ', '')) > 0)`

const receivedOpenReportSQL = `SELECT DISTINCT
	c.ID_Pengaduan,
	d.Kantor_Tujuan_Update,
	d.Kantor_Asal_Update,
	COALESCE(c.Nomor_Kiriman, ''),
	COALESCE(c.Jenis_Kiriman, ''),
	c.Tanggal_Tambah,
	COALESCE(c.Kantor_Tujuan, ''),
	h.Status_Update,
	COALESCE(s.Deskripsi_Status, ''),
	h.Kantor_Asal_Update,
	h.Tanggal_Update,
	GREATEST(TIMESTAMPDIFF(SECOND, c.Tanggal_Tambah, CURRENT_TIMESTAMP), 0)
FROM cchentridet AS d
INNER JOIN cchentri AS c ON c.ID_Pengaduan = d.ID_Pengaduan
INNER JOIN cchentrimas AS h
	ON h.ID_Pengaduan = c.ID_Pengaduan
	AND h.ID_Histori = (
		SELECT latest.ID_Histori
		FROM cchentrimas AS latest
		WHERE latest.ID_Pengaduan = c.ID_Pengaduan
		ORDER BY latest.Tanggal_Update DESC, latest.ID_Histori DESC
		LIMIT 1
	)
LEFT JOIN cch_jenis_penanganan AS s ON s.ID_Status = h.Status_Update
WHERE d.Kantor_Tujuan_Update = ?
	AND d.Status_Update = '101'
	AND c.Status_Akhir <> '109'
	AND h.Status_Update <> '109'
	AND c.Tanggal_Tambah BETWEEN ? AND ?`

const receivedOpenDurationBucketsSQL = `SELECT
	COALESCE(SUM(CASE WHEN eligible.age_days = 0 THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN eligible.age_days = 1 THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN eligible.age_days = 2 THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN eligible.age_days = 3 THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN eligible.age_days >= 4 THEN 1 ELSE 0 END), 0)
FROM (
	SELECT DISTINCT c.ID_Pengaduan, DATEDIFF(CURRENT_DATE, DATE(c.Tanggal_Tambah)) AS age_days
	FROM cchentridet AS d
	INNER JOIN cchentri AS c ON c.ID_Pengaduan = d.ID_Pengaduan
	WHERE d.Kantor_Tujuan_Update = ?
		AND d.Status_Update = '101'
		AND c.Status_Akhir <> '109'
		AND FIND_IN_SET(?, REPLACE(COALESCE(c.Semua_Tujuan, ''), ' ', '')) > 0
		AND c.Tanggal_Tambah BETWEEN ? AND ?`

const sentOpenDurationBucketsSQL = `SELECT
	COALESCE(SUM(CASE WHEN eligible.age_days = 0 THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN eligible.age_days = 1 THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN eligible.age_days = 2 THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN eligible.age_days = 3 THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN eligible.age_days >= 4 THEN 1 ELSE 0 END), 0)
FROM (
	SELECT DISTINCT c.ID_Pengaduan, DATEDIFF(CURRENT_DATE, DATE(c.Tanggal_Tambah)) AS age_days
	FROM cchentridet AS d
	INNER JOIN cchentri AS c ON c.ID_Pengaduan = d.ID_Pengaduan
	INNER JOIN cchentrimas AS h
		ON h.ID_Pengaduan = c.ID_Pengaduan
		AND h.ID_Histori = (
			SELECT latest.ID_Histori
			FROM cchentrimas AS latest
			WHERE latest.ID_Pengaduan = c.ID_Pengaduan
			ORDER BY latest.Tanggal_Update DESC, latest.ID_Histori DESC
			LIMIT 1
		)
	WHERE c.Kantor_Asal = ?
		AND d.Status_Update = '101'
		AND h.Status_Update <> '109'
		AND c.Tanggal_Tambah BETWEEN ? AND ?`

const receivedOpenComplaintCountSQL = `SELECT COUNT(*)
FROM cchentri
WHERE Status_Akhir <> '109'
AND FIND_IN_SET(?, REPLACE(COALESCE(Semua_Tujuan, ''), ' ', '')) > 0`

const sentOpenComplaintCountSQL = `SELECT COUNT(*)
FROM cchentri
WHERE Status_Akhir <> '109'
AND Kantor_Asal = ?`

const sentSummarySQL = `SELECT
	e.Kantor_Asal AS OfficeCode,
	k.Nama_Kantor AS OfficeName,
	COUNT(e.ID_Pengaduan) AS Total,
	COALESCE(SUM(e.Status_Akhir = '109'), 0) AS Completed,
	COALESCE(SUM(e.Status_Akhir = '101'), 0) AS Unresponded
FROM cchentri AS e
INNER JOIN cch_kantor AS k ON k.Kode_Kantor = e.Kantor_Asal
WHERE e.Tanggal_Tambah BETWEEN ? AND ?`

const receivedSummarySQL = `SELECT
	t.OfficeCode,
	k.Nama_Kantor AS OfficeName,
	COUNT(DISTINCT t.ID_Pengaduan) AS Total,
	COALESCE(SUM(t.Completed = 1), 0) AS Completed,
	COALESCE(SUM(t.MaxStatus = '101'), 0) AS Unresponded
FROM (
	SELECT DISTINCT
		u.Kantor_Tujuan_Update AS OfficeCode,
		e.ID_Pengaduan,
		CASE WHEN e.Status_Akhir = '109' THEN 1 ELSE 0 END AS Completed,
		(SELECT MAX(st.Status_Update)
		 FROM cchentridet AS st
		 WHERE st.ID_Pengaduan = e.ID_Pengaduan
		   AND st.Kantor_Tujuan_Update = u.Kantor_Tujuan_Update
		   AND st.Status_Update <> '109') AS MaxStatus
	FROM cchentridet AS u
	INNER JOIN cchentri AS e ON e.ID_Pengaduan = u.ID_Pengaduan
	WHERE u.Status_Update = '101'
		AND e.Tanggal_Tambah BETWEEN ? AND ?`

const sentBreakdownShipmentSQL = `SELECT
	e.Jenis_Kiriman AS Code,
	l.Deskripsi AS Label,
	COUNT(e.ID_Pengaduan) AS Count
FROM cchentri AS e
INNER JOIN cch_jenis_layanan AS l ON l.Kode_Layanan = e.Jenis_Kiriman
WHERE e.Tanggal_Tambah BETWEEN ? AND ?`

const sentBreakdownProblemSQL = `SELECT
	e.Jenis_Pengaduan AS Code,
	l.Deskripsi AS Label,
	COUNT(e.ID_Pengaduan) AS Count
FROM cchentri AS e
INNER JOIN cch_jenis_pengaduan AS l ON l.Kode_Jenis = e.Jenis_Pengaduan AND l.Status_Aktif = 'Y'
WHERE e.Tanggal_Tambah BETWEEN ? AND ?`

const receivedBreakdownShipmentSQL = `SELECT
	e.Jenis_Kiriman AS Code,
	l.Deskripsi AS Label,
	COUNT(e.ID_Pengaduan) AS Count
FROM cchentridet AS u
INNER JOIN cchentri AS e ON e.ID_Pengaduan = u.ID_Pengaduan
INNER JOIN cch_jenis_layanan AS l ON l.Kode_Layanan = e.Jenis_Kiriman
WHERE u.Status_Update = '101'
	AND e.Tanggal_Tambah BETWEEN ? AND ?`

const receivedBreakdownProblemSQL = `SELECT
	e.Jenis_Pengaduan AS Code,
	l.Deskripsi AS Label,
	COUNT(e.ID_Pengaduan) AS Count
FROM cchentridet AS u
INNER JOIN cchentri AS e ON e.ID_Pengaduan = u.ID_Pengaduan
INNER JOIN cch_jenis_pengaduan AS l ON l.Kode_Jenis = e.Jenis_Pengaduan AND l.Status_Aktif = 'Y'
WHERE u.Status_Update = '101'
	AND e.Tanggal_Tambah BETWEEN ? AND ?`

const sentOpenReportSQL = `SELECT DISTINCT
	c.ID_Pengaduan,
	d.Kantor_Asal_Update,
	COALESCE(c.Nomor_Kiriman, ''),
	COALESCE(c.Jenis_Kiriman, ''),
	c.Tanggal_Tambah,
	COALESCE(c.Kantor_Tujuan, ''),
	h.Status_Update,
	COALESCE(s.Deskripsi_Status, ''),
	h.Kantor_Asal_Update,
	h.Tanggal_Update,
	GREATEST(TIMESTAMPDIFF(SECOND, c.Tanggal_Tambah, CURRENT_TIMESTAMP), 0)
FROM cchentridet AS d
INNER JOIN cchentri AS c ON c.ID_Pengaduan = d.ID_Pengaduan
INNER JOIN cchentrimas AS h
	ON h.ID_Pengaduan = c.ID_Pengaduan
	AND h.ID_Histori = (
		SELECT latest.ID_Histori
		FROM cchentrimas AS latest
		WHERE latest.ID_Pengaduan = c.ID_Pengaduan
		ORDER BY latest.Tanggal_Update DESC, latest.ID_Histori DESC
		LIMIT 1
	)
LEFT JOIN cch_jenis_penanganan AS s ON s.ID_Status = h.Status_Update
WHERE c.Kantor_Asal = ?
	AND d.Status_Update = '101'
	AND h.Status_Update <> '109'
	AND c.Tanggal_Tambah BETWEEN ? AND ?`

const complaintDetailQuerySQL = `SELECT
	c.ID_Pengaduan,
	c.Nama_Pelanggan,
	COALESCE(c.Email_Pelanggan, ''),
	c.Nomor_HP_Pelanggan,
	COALESCE(c.Alamat_Pelanggan, ''),
	c.Kantor_Asal,
	c.Kantor_Tujuan,
	COALESCE(c.Kantor_Tembusan, ''),
	COALESCE(c.Semua_Tujuan, ''),
	c.Jenis_Pengaduan,
	c.Sumber_Pengaduan,
	c.Jenis_Kiriman,
	c.Nomor_Kiriman,
	c.Tanggal_Pengaduan,
	COALESCE(c.Kantor_Asal_Kiriman, ''),
	c.Kantor_Tujuan_Kiriman,
	c.Ringkasan_Pengaduan,
	c.Uraian_Pengaduan,
	c.Status_Akhir,
	COALESCE(s.Deskripsi_Status, ''),
	c.Tanggal_Status,
	c.Kantor_Status,
	COALESCE(c.Request_Status, '0')
FROM cchentri AS c
LEFT JOIN cch_jenis_penanganan AS s ON s.ID_Status = c.Status_Akhir
WHERE c.ID_Pengaduan = ?
	AND c.Status_Akhir <> '109'
	AND (c.Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Semua_Tujuan, ''), ' ', '')) > 0)
LIMIT 1`

const completedComplaintDetailQuerySQL = `SELECT
	c.ID_Pengaduan,
	c.Nama_Pelanggan,
	COALESCE(c.Email_Pelanggan, ''),
	c.Nomor_HP_Pelanggan,
	COALESCE(c.Alamat_Pelanggan, ''),
	c.Kantor_Asal,
	c.Kantor_Tujuan,
	COALESCE(c.Kantor_Tembusan, ''),
	COALESCE(c.Semua_Tujuan, ''),
	c.Jenis_Pengaduan,
	c.Sumber_Pengaduan,
	c.Jenis_Kiriman,
	c.Nomor_Kiriman,
	c.Tanggal_Pengaduan,
	COALESCE(c.Kantor_Asal_Kiriman, ''),
	c.Kantor_Tujuan_Kiriman,
	c.Ringkasan_Pengaduan,
	c.Uraian_Pengaduan,
	c.Status_Akhir,
	COALESCE(s.Deskripsi_Status, ''),
	c.Tanggal_Status,
	c.Kantor_Status,
	COALESCE(c.Request_Status, '0')
FROM cchentri AS c
LEFT JOIN cch_jenis_penanganan AS s ON s.ID_Status = c.Status_Akhir
WHERE c.ID_Pengaduan = ?
	AND c.Status_Akhir = '109'
	AND (c.Kantor_Asal = ? OR c.Kantor_Tujuan = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Kantor_Tembusan, ''), ' ', '')) > 0)
LIMIT 1`

const callCenterStatusOptionsQuerySQL = `SELECT
	s.ID_Status,
	COALESCE(s.Deskripsi_Status, '')
FROM cchentri AS c
LEFT JOIN cch_jenis_penanganan AS s
	ON s.Aktif = 'Y' AND s.ID_Status IN ('103', '109')
WHERE c.ID_Pengaduan = ?
	AND c.Kantor_Asal = ?
	AND c.Status_Akhir <> '109'
ORDER BY s.ID_Status ASC`

var ErrRepositoryNotConfigured = errors.New("complaint repository is not configured")

type MySQLComplaintRepository struct {
	db *sql.DB
}

var _ complaints.ComplaintRepository = (*MySQLComplaintRepository)(nil)

func NewMySQLComplaintRepository(db *sql.DB) *MySQLComplaintRepository {
	return &MySQLComplaintRepository{db: db}
}

func (repository *MySQLComplaintRepository) ListInvolvingOffice(ctx context.Context, query complaints.ComplaintListQuery) ([]complaints.ComplaintSummary, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return nil, err
	}

	statement, args := buildComplaintListQuery(query)
	rows, err := repository.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("list complaints involving office: %w", err)
	}
	defer rows.Close()

	complaintsFound := make([]complaints.ComplaintSummary, 0, query.Limit)
	for rows.Next() {
		complaint, err := scanComplaintSummary(rows)
		if err != nil {
			return nil, fmt.Errorf("scan complaint summary: %w", err)
		}
		complaintsFound = append(complaintsFound, complaint)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read complaint summaries: %w", err)
	}
	return complaintsFound, nil
}

func (repository *MySQLComplaintRepository) ListReceivedOpenReport(ctx context.Context, query complaints.ReceivedOpenReportQuery, officeCode string) ([]complaints.ReceivedOpenReportRow, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	if strings.TrimSpace(officeCode) == "" || len(officeCode) > 7 || strings.Contains(officeCode, ",") {
		return nil, complaints.ErrInvalidComplaintListQuery
	}
	query, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	statement := receivedOpenReportSQL
	args := []any{officeCode, *query.From, *query.To}
	if query.ProductCode != "" {
		statement += "\n\tAND c.Jenis_Kiriman = ?"
		args = append(args, query.ProductCode)
	}
	statement += "\nORDER BY c.Tanggal_Tambah DESC, c.ID_Pengaduan DESC LIMIT ? OFFSET ?"
	args = append(args, query.Limit, query.Offset)
	rows, err := repository.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("list received-open complaint report: %w", err)
	}
	defer rows.Close()
	reportRows := make([]complaints.ReceivedOpenReportRow, 0, query.Limit)
	for rows.Next() {
		var row complaints.ReceivedOpenReportRow
		if err := rows.Scan(
			&row.ComplaintID,
			&row.TargetOfficeUpdateCode,
			&row.OriginOfficeUpdateCode,
			&row.ShipmentNumber,
			&row.ShipmentTypeCode,
			&row.ComplaintCreatedAt,
			&row.ShipmentDestinationOfficeCode,
			&row.LatestStatusCode,
			&row.LatestStatusDescription,
			&row.LatestStatusOfficeCode,
			&row.LatestStatusAt,
			&row.DurationSeconds,
		); err != nil {
			return nil, fmt.Errorf("scan received-open complaint report row: %w", err)
		}
		reportRows = append(reportRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read received-open complaint report: %w", err)
	}
	return reportRows, nil
}

func (repository *MySQLComplaintRepository) ListSentOpenReport(ctx context.Context, query complaints.SentOpenReportQuery, officeCode string) ([]complaints.SentOpenReportRow, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	if strings.TrimSpace(officeCode) == "" || len(officeCode) > 7 || strings.Contains(officeCode, ",") {
		return nil, complaints.ErrInvalidComplaintListQuery
	}
	query, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	statement := sentOpenReportSQL
	args := []any{officeCode, *query.From, *query.To}
	if query.ProductCode != "" {
		statement += "\n\tAND c.Jenis_Kiriman = ?"
		args = append(args, query.ProductCode)
	}
	statement += "\nORDER BY c.Tanggal_Tambah DESC, c.ID_Pengaduan DESC LIMIT ? OFFSET ?"
	args = append(args, query.Limit, query.Offset)
	rows, err := repository.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("list sent-open complaint report: %w", err)
	}
	defer rows.Close()
	reportRows := make([]complaints.SentOpenReportRow, 0, query.Limit)
	for rows.Next() {
		var row complaints.SentOpenReportRow
		if err := rows.Scan(
			&row.ComplaintID,
			&row.OriginOfficeUpdateCode,
			&row.ShipmentNumber,
			&row.ShipmentTypeCode,
			&row.ComplaintCreatedAt,
			&row.ShipmentDestinationOfficeCode,
			&row.LatestStatusCode,
			&row.LatestStatusDescription,
			&row.LatestStatusOfficeCode,
			&row.LatestStatusAt,
			&row.DurationSeconds,
		); err != nil {
			return nil, fmt.Errorf("scan sent-open complaint report row: %w", err)
		}
		reportRows = append(reportRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read sent-open complaint report: %w", err)
	}
	return reportRows, nil
}

func (repository *MySQLComplaintRepository) ListSentSummary(ctx context.Context, query complaints.SentSummaryQuery, officeCode string) ([]complaints.SentSummaryRow, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	if strings.TrimSpace(officeCode) == "" || len(officeCode) > 7 || strings.Contains(officeCode, ",") {
		return nil, complaints.ErrInvalidComplaintListQuery
	}
	query, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	statement := sentSummarySQL
	args := []any{*query.From, *query.To}
	if query.ProductCode != "" {
		statement += "\n\tAND e.Jenis_Kiriman = ?"
		args = append(args, query.ProductCode)
	}
	if query.RegionalCode != "" {
		statement += "\n\tAND ((k.Regional = ?) OR (k.Kode_Kantor = ?))"
		args = append(args, query.RegionalCode, query.RegionalCode)
	}
	statement += "\nGROUP BY e.Kantor_Asal, k.Nama_Kantor\nORDER BY e.Kantor_Asal ASC\nLIMIT ? OFFSET ?"
	args = append(args, query.Limit, query.Offset)
	rows, err := repository.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("list sent complaint summary: %w", err)
	}
	defer rows.Close()
	summary := make([]complaints.SentSummaryRow, 0, query.Limit)
	for rows.Next() {
		var row complaints.SentSummaryRow
		if err := rows.Scan(&row.OfficeCode, &row.OfficeName, &row.Total, &row.Completed, &row.Unresponded); err != nil {
			return nil, fmt.Errorf("scan sent complaint summary row: %w", err)
		}
		row.Open = row.Total - row.Completed
		row.Responded = row.Total - row.Unresponded
		summary = append(summary, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read sent complaint summary: %w", err)
	}
	return summary, nil
}

func (repository *MySQLComplaintRepository) ListReceivedSummary(ctx context.Context, query complaints.ReceivedSummaryQuery, officeCode string) ([]complaints.ReceivedSummaryRow, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	if strings.TrimSpace(officeCode) == "" || len(officeCode) > 7 || strings.Contains(officeCode, ",") {
		return nil, complaints.ErrInvalidComplaintListQuery
	}
	query, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	statement := receivedSummarySQL
	args := []any{*query.From, *query.To}
	if query.ProductCode != "" {
		statement += "\n\t\tAND e.Jenis_Kiriman = ?"
		args = append(args, query.ProductCode)
	}
	if query.OnlyTarget {
		statement += "\n\t\tAND e.Kantor_Tujuan = u.Kantor_Tujuan_Update"
	}
	statement += "\n) AS t\nINNER JOIN cch_kantor AS k ON k.Kode_Kantor = t.OfficeCode"
	if query.RegionalCode != "" {
		statement += "\n\tAND ((k.Regional = ?) OR (k.Kode_Kantor = ?))"
		args = append(args, query.RegionalCode, query.RegionalCode)
	}
	statement += "\nGROUP BY t.OfficeCode, k.Nama_Kantor\nORDER BY t.OfficeCode ASC\nLIMIT ? OFFSET ?"
	args = append(args, query.Limit, query.Offset)
	rows, err := repository.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("list received complaint summary: %w", err)
	}
	defer rows.Close()
	summary := make([]complaints.ReceivedSummaryRow, 0, query.Limit)
	for rows.Next() {
		var row complaints.ReceivedSummaryRow
		if err := rows.Scan(&row.OfficeCode, &row.OfficeName, &row.Total, &row.Completed, &row.Unresponded); err != nil {
			return nil, fmt.Errorf("scan received complaint summary row: %w", err)
		}
		if row.Completed > row.Total {
			row.Completed = row.Total
		}
		if row.Unresponded > row.Total {
			row.Unresponded = row.Total
		}
		row.Open = row.Total - row.Completed
		row.Responded = row.Total - row.Unresponded
		summary = append(summary, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read received complaint summary: %w", err)
	}
	return summary, nil
}

func (repository *MySQLComplaintRepository) ListBreakdown(ctx context.Context, query complaints.BreakdownQuery, officeCode string) ([]complaints.BreakdownRow, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	if strings.TrimSpace(officeCode) == "" || len(officeCode) > 7 || strings.Contains(officeCode, ",") {
		return nil, complaints.ErrInvalidComplaintListQuery
	}
	query, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	statement := ""
	args := []any{*query.From, *query.To}
	switch query.Direction {
	case complaints.BreakdownDirectionSent:
		if query.Kind == complaints.BreakdownKindShipment {
			statement = sentBreakdownShipmentSQL
		} else {
			statement = sentBreakdownProblemSQL
		}
		if query.ProductCode != "" {
			statement += "\n\tAND e.Jenis_Kiriman = ?"
			args = append(args, query.ProductCode)
		}
		if query.RegionalCode != "" {
			statement += "\n\tAND e.Kantor_Asal IN (SELECT k.Kode_Kantor FROM cch_kantor AS k WHERE k.Regional = ? OR k.Kode_Kantor = ?)"
			args = append(args, query.RegionalCode, query.RegionalCode)
		}
	case complaints.BreakdownDirectionReceived:
		if query.Kind == complaints.BreakdownKindShipment {
			statement = receivedBreakdownShipmentSQL
		} else {
			statement = receivedBreakdownProblemSQL
		}
		if query.OnlyTarget {
			statement += "\n\tAND e.Kantor_Tujuan = u.Kantor_Tujuan_Update"
		}
		if query.ProductCode != "" {
			statement += "\n\tAND e.Jenis_Kiriman = ?"
			args = append(args, query.ProductCode)
		}
		if query.RegionalCode != "" {
			statement += "\n\tAND u.Kantor_Tujuan_Update IN (SELECT k.Kode_Kantor FROM cch_kantor AS k WHERE k.Regional = ? OR k.Kode_Kantor = ?)"
			args = append(args, query.RegionalCode, query.RegionalCode)
		}
	}
	statement += "\nGROUP BY Code, Label\nORDER BY Count DESC, Code ASC\nLIMIT ? OFFSET ?"
	args = append(args, query.Limit, query.Offset)
	rows, err := repository.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("list complaint breakdown: %w", err)
	}
	defer rows.Close()
	breakdown := make([]complaints.BreakdownRow, 0, query.Limit)
	for rows.Next() {
		var row complaints.BreakdownRow
		if err := rows.Scan(&row.Code, &row.Label, &row.Count); err != nil {
			return nil, fmt.Errorf("scan complaint breakdown row: %w", err)
		}
		breakdown = append(breakdown, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read complaint breakdown: %w", err)
	}
	return breakdown, nil
}

func (repository *MySQLComplaintRepository) CountSentOpenDurationBuckets(ctx context.Context, query complaints.SentOpenDurationQuery, officeCode string) (complaints.SentOpenDurationBuckets, error) {
	if repository == nil || repository.db == nil {
		return complaints.SentOpenDurationBuckets{}, ErrRepositoryNotConfigured
	}
	if strings.TrimSpace(officeCode) == "" || len(officeCode) > 7 || strings.Contains(officeCode, ",") {
		return complaints.SentOpenDurationBuckets{}, complaints.ErrInvalidComplaintListQuery
	}
	query, err := query.Normalize()
	if err != nil {
		return complaints.SentOpenDurationBuckets{}, err
	}
	statement := sentOpenDurationBucketsSQL
	args := []any{officeCode, *query.From, *query.To}
	if query.ProductCode != "" {
		statement += "\n\t\tAND c.Jenis_Kiriman = ?"
		args = append(args, query.ProductCode)
	}
	statement += "\n) AS eligible"
	var buckets complaints.SentOpenDurationBuckets
	if err := repository.db.QueryRowContext(ctx, statement, args...).Scan(
		&buckets.DayZero,
		&buckets.DayOne,
		&buckets.DayTwo,
		&buckets.DayThree,
		&buckets.DayFourPlus,
	); err != nil {
		return complaints.SentOpenDurationBuckets{}, fmt.Errorf("count sent-open duration buckets: %w", err)
	}
	return buckets, nil
}

func (repository *MySQLComplaintRepository) CountReceivedOpenDurationBuckets(ctx context.Context, query complaints.ReceivedOpenDurationQuery, officeCode string) (complaints.ReceivedOpenDurationBuckets, error) {
	if repository == nil || repository.db == nil {
		return complaints.ReceivedOpenDurationBuckets{}, ErrRepositoryNotConfigured
	}
	if strings.TrimSpace(officeCode) == "" || len(officeCode) > 7 || strings.Contains(officeCode, ",") {
		return complaints.ReceivedOpenDurationBuckets{}, complaints.ErrInvalidComplaintListQuery
	}
	query, err := query.Normalize()
	if err != nil {
		return complaints.ReceivedOpenDurationBuckets{}, err
	}
	statement := receivedOpenDurationBucketsSQL
	args := []any{officeCode, officeCode, *query.From, *query.To}
	if query.ProductCode != "" {
		statement += "\n\t\tAND c.Jenis_Kiriman = ?"
		args = append(args, query.ProductCode)
	}
	statement += "\n) AS eligible"
	var buckets complaints.ReceivedOpenDurationBuckets
	if err := repository.db.QueryRowContext(ctx, statement, args...).Scan(
		&buckets.DayZero,
		&buckets.DayOne,
		&buckets.DayTwo,
		&buckets.DayThree,
		&buckets.DayFourPlus,
	); err != nil {
		return complaints.ReceivedOpenDurationBuckets{}, fmt.Errorf("count received-open duration buckets: %w", err)
	}
	return buckets, nil
}

func (repository *MySQLComplaintRepository) ListOpenTicketHistory(ctx context.Context, query complaints.ComplaintHistoryQuery) ([]complaints.ComplaintStatusHistoryEntry, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	rows, err := repository.db.QueryContext(ctx, complaintHistoryQuerySQL,
		query.ComplaintID,
		query.OfficeCode,
		query.OfficeCode,
		query.Limit,
		query.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list open complaint history: %w", err)
	}
	defer rows.Close()

	history := make([]complaints.ComplaintStatusHistoryEntry, 0, query.Limit)
	for rows.Next() {
		entry, err := scanComplaintStatusHistory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan complaint status history: %w", err)
		}
		history = append(history, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read complaint status history: %w", err)
	}
	return history, nil
}

func (repository *MySQLComplaintRepository) ListCompletedTicketHistory(ctx context.Context, query complaints.ComplaintHistoryQuery) ([]complaints.ComplaintStatusHistoryEntry, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	rows, err := repository.db.QueryContext(ctx, completedComplaintHistoryQuerySQL,
		query.ComplaintID, query.OfficeCode, query.OfficeCode, query.OfficeCode, query.Limit, query.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list completed complaint history: %w", err)
	}
	defer rows.Close()
	history := make([]complaints.ComplaintStatusHistoryEntry, 0, query.Limit)
	for rows.Next() {
		entry, err := scanComplaintStatusHistory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan completed complaint history: %w", err)
		}
		history = append(history, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read completed complaint history: %w", err)
	}
	return history, nil
}

func (repository *MySQLComplaintRepository) CountCompletedForCallCenter(ctx context.Context, query complaints.CompletedCountQuery, officeCode string) (int64, error) {
	if repository == nil || repository.db == nil {
		return 0, ErrRepositoryNotConfigured
	}
	if strings.TrimSpace(officeCode) == "" || len(officeCode) > 7 || strings.Contains(officeCode, ",") {
		return 0, complaints.ErrInvalidComplaintListQuery
	}
	query, err := query.Normalize()
	if err != nil {
		return 0, err
	}
	statement := completedComplaintCountSQL
	args := []any{officeCode, officeCode}
	if query.From != nil {
		statement += " AND Tanggal_Pengaduan BETWEEN ? AND ?"
		args = append(args, *query.From, *query.To)
	}
	var count int64
	if err := repository.db.QueryRowContext(ctx, statement, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count completed complaints: %w", err)
	}
	return count, nil
}

func (repository *MySQLComplaintRepository) CountOpenForCallCenter(ctx context.Context, query complaints.OpenCountQuery, officeCode string) (int64, error) {
	if repository == nil || repository.db == nil {
		return 0, ErrRepositoryNotConfigured
	}
	if strings.TrimSpace(officeCode) == "" || len(officeCode) > 7 || strings.Contains(officeCode, ",") {
		return 0, complaints.ErrInvalidComplaintListQuery
	}
	query, err := query.Normalize()
	if err != nil {
		return 0, err
	}
	statement := openComplaintCountSQL
	args := []any{officeCode, officeCode}
	if query.From != nil {
		statement += " AND Tanggal_Pengaduan BETWEEN ? AND ?"
		args = append(args, *query.From, *query.To)
	}
	var count int64
	if err := repository.db.QueryRowContext(ctx, statement, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count open complaints: %w", err)
	}
	return count, nil
}

func (repository *MySQLComplaintRepository) CountReceivedOpenForCallCenter(ctx context.Context, query complaints.OpenCountQuery, officeCode string) (int64, error) {
	return repository.countDashboardComplaints(ctx, receivedOpenComplaintCountSQL, "count received open complaints", query, officeCode)
}

func (repository *MySQLComplaintRepository) CountSentOpenForCallCenter(ctx context.Context, query complaints.OpenCountQuery, officeCode string) (int64, error) {
	return repository.countDashboardComplaints(ctx, sentOpenComplaintCountSQL, "count sent open complaints", query, officeCode)
}

func (repository *MySQLComplaintRepository) countDashboardComplaints(ctx context.Context, baseStatement, operation string, query complaints.OpenCountQuery, officeCode string) (int64, error) {
	if repository == nil || repository.db == nil {
		return 0, ErrRepositoryNotConfigured
	}
	if strings.TrimSpace(officeCode) == "" || len(officeCode) > 7 || strings.Contains(officeCode, ",") {
		return 0, complaints.ErrInvalidComplaintListQuery
	}
	query, err := query.Normalize()
	if err != nil {
		return 0, err
	}
	args := []any{officeCode}
	statement := baseStatement
	if query.From != nil {
		statement += " AND Tanggal_Pengaduan BETWEEN ? AND ?"
		args = append(args, *query.From, *query.To)
	}
	var count int64
	if err := repository.db.QueryRowContext(ctx, statement, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("%s: %w", operation, err)
	}
	return count, nil
}

func (repository *MySQLComplaintRepository) GetOpenTicketForOffice(ctx context.Context, query complaints.ComplaintDetailQuery) (complaints.ComplaintDetail, error) {
	if repository == nil || repository.db == nil {
		return complaints.ComplaintDetail{}, ErrRepositoryNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return complaints.ComplaintDetail{}, err
	}
	row := repository.db.QueryRowContext(ctx, complaintDetailQuerySQL, query.ComplaintID, query.OfficeCode, query.OfficeCode)
	detail, err := scanComplaintDetail(row)
	if err != nil {
		return complaints.ComplaintDetail{}, fmt.Errorf("get open complaint detail: %w", err)
	}
	return detail, nil
}

func (repository *MySQLComplaintRepository) GetCompletedTicketForOffice(ctx context.Context, query complaints.ComplaintDetailQuery) (complaints.ComplaintDetail, error) {
	if repository == nil || repository.db == nil {
		return complaints.ComplaintDetail{}, ErrRepositoryNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return complaints.ComplaintDetail{}, err
	}
	row := repository.db.QueryRowContext(ctx, completedComplaintDetailQuerySQL, query.ComplaintID, query.OfficeCode, query.OfficeCode, query.OfficeCode)
	detail, err := scanComplaintDetail(row)
	if err != nil {
		return complaints.ComplaintDetail{}, fmt.Errorf("get completed complaint detail: %w", err)
	}
	return detail, nil
}

func (repository *MySQLComplaintRepository) ListCallCenterStatusOptions(ctx context.Context, complaintID string, officeCode string) ([]complaints.ComplaintStatusOption, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, callCenterStatusOptionsQuerySQL, complaintID, officeCode)
	if err != nil {
		return nil, fmt.Errorf("list Call Center complaint status options: %w", err)
	}
	defer rows.Close()
	options := make([]complaints.ComplaintStatusOption, 0, 2)
	parentFound := false
	for rows.Next() {
		parentFound = true
		var code sql.NullString
		var description string
		if err := rows.Scan(&code, &description); err != nil {
			return nil, fmt.Errorf("scan Call Center complaint status option: %w", err)
		}
		if code.Valid {
			options = append(options, complaints.ComplaintStatusOption{Code: code.String, Description: description})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read Call Center complaint status options: %w", err)
	}
	if !parentFound {
		return nil, sql.ErrNoRows
	}
	return options, nil
}

func buildComplaintListQuery(query complaints.ComplaintListQuery) (string, []any) {
	var officePredicate string
	args := make([]any, 0, 8)
	switch query.OfficeMatch {
	case complaints.MatchOriginOrAllTargets:
		officePredicate = "(Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(Semua_Tujuan, ''), ' ', '')) > 0)"
		args = append(args, query.OfficeCode, query.OfficeCode)
	case complaints.MatchAllTargetsOnly:
		officePredicate = "FIND_IN_SET(?, REPLACE(COALESCE(Semua_Tujuan, ''), ' ', '')) > 0"
		args = append(args, query.OfficeCode)
	case complaints.MatchOriginOnly:
		officePredicate = "Kantor_Asal = ?"
		args = append(args, query.OfficeCode)
	default:
		officePredicate = "(Kantor_Asal = ? OR Kantor_Tujuan = ? OR FIND_IN_SET(?, REPLACE(COALESCE(Kantor_Tembusan, ''), ' ', '')) > 0 OR FIND_IN_SET(?, REPLACE(COALESCE(Semua_Tujuan, ''), ' ', '')) > 0)"
		args = append(args, query.OfficeCode, query.OfficeCode, query.OfficeCode, query.OfficeCode)
	}
	if query.OpenOnly {
		officePredicate += " AND Status_Akhir <> '109'"
	} else if query.CompletedOnly {
		officePredicate += " AND Status_Akhir = '109'"
	}
	if query.From != nil {
		officePredicate += " AND Tanggal_Pengaduan BETWEEN ? AND ?"
		args = append(args, *query.From, *query.To)
	}
	args = append(args, query.Limit, query.Offset)
	return complaintListSelectSQL + "\nWHERE " + officePredicate + "\n" + complaintListOrderSQL, args
}

func scanComplaintSummary(rows *sql.Rows) (complaints.ComplaintSummary, error) {
	var complaint complaints.ComplaintSummary
	var copyOfficeCodes sql.NullString
	var allTargetOfficeCodes sql.NullString
	var complaintDate sql.NullTime
	if err := rows.Scan(
		&complaint.ComplaintID,
		&complaint.OriginOfficeCode,
		&complaint.DestinationOfficeCode,
		&copyOfficeCodes,
		&allTargetOfficeCodes,
		&complaint.ShipmentNumber,
		&complaintDate,
		&complaint.Summary,
		&complaint.StatusCode,
		&complaint.StatusDate,
		&complaint.StatusOfficeCode,
	); err != nil {
		return complaints.ComplaintSummary{}, err
	}
	if copyOfficeCodes.Valid {
		complaint.CopyOfficeCodes = copyOfficeCodes.String
	}
	if allTargetOfficeCodes.Valid {
		complaint.AllTargetOfficeCodes = allTargetOfficeCodes.String
	}
	if complaintDate.Valid {
		date := complaintDate.Time
		complaint.ComplaintDate = &date
	}
	return complaint, nil
}

func scanComplaintStatusHistory(rows *sql.Rows) (complaints.ComplaintStatusHistoryEntry, error) {
	var entry complaints.ComplaintStatusHistoryEntry
	var durationSeconds sql.NullInt64
	if err := rows.Scan(
		&entry.ComplaintID,
		&entry.HistoryID,
		&entry.StatusCode,
		&entry.StatusDescription,
		&entry.OriginOfficeCode,
		&entry.Description,
		&entry.UpdatedAt,
		&entry.UpdatedBy,
		&durationSeconds,
	); err != nil {
		return complaints.ComplaintStatusHistoryEntry{}, err
	}
	if durationSeconds.Valid {
		entry.DurationSeconds = &durationSeconds.Int64
	}
	return entry, nil
}

func scanComplaintDetail(row *sql.Row) (complaints.ComplaintDetail, error) {
	var detail complaints.ComplaintDetail
	var customerEmail sql.NullString
	var complaintDate sql.NullTime
	var durationStatus sql.NullString
	if err := row.Scan(
		&detail.ComplaintID,
		&detail.CustomerName,
		&customerEmail,
		&detail.CustomerPhone,
		&detail.CustomerAddress,
		&detail.OriginOfficeCode,
		&detail.DestinationOfficeCode,
		&detail.CopyOfficeCodes,
		&detail.AllTargetOfficeCodes,
		&detail.ComplaintTypeCode,
		&detail.ComplaintSourceCode,
		&detail.ShipmentTypeCode,
		&detail.ShipmentNumber,
		&complaintDate,
		&detail.ShipmentOriginOffice,
		&detail.ShipmentTargetOffice,
		&detail.Summary,
		&detail.Description,
		&detail.StatusCode,
		&durationStatus,
		&detail.StatusDate,
		&detail.StatusOfficeCode,
		&detail.RequestStatus,
	); err != nil {
		return complaints.ComplaintDetail{}, err
	}
	if customerEmail.Valid {
		detail.CustomerEmail = customerEmail.String
	}
	if complaintDate.Valid {
		date := complaintDate.Time
		detail.ComplaintDate = &date
	}
	if durationStatus.Valid {
		detail.StatusDescription = durationStatus.String
	}
	return detail, nil
}
