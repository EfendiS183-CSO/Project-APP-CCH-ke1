package repository

import (
	"context"
	"database/sql"
	"fmt"

	"cch.local/backend/internal/modules/complaints"
)

const activeComplaintTypesQuery = `SELECT Kode_Jenis, Deskripsi
FROM cch_jenis_pengaduan
WHERE Status_Aktif = 'Y'
ORDER BY Kode_Jenis ASC`

const activeComplaintSourcesQuery = `SELECT ID, Sumber
FROM cch_sumber_pengaduan
WHERE Apakah_Aktif = 'Y'
ORDER BY ID ASC`

const destinationOfficesQuery = `SELECT Kode_Kantor, Nama_Kantor
FROM cch_kantor
WHERE Sebagai_Tujuan = 'Y'
	AND Apakah_Aktif = 'Y'
	AND Kode_Kantor <> ?
ORDER BY Kode_Kantor ASC`

const copyOfficesQuery = `SELECT Kode_Kantor, Nama_Kantor
FROM cch_kantor
WHERE Sebagai_Tembusan = 'Y'
	AND Apakah_Aktif = 'Y'
	AND Kode_Kantor NOT IN ('40005CC', '40005PI', ?)
ORDER BY Kode_Kantor ASC`

const shippingServicesQuery = `SELECT Kode_Layanan, Deskripsi
FROM cch_jenis_layanan
ORDER BY Kode_Layanan ASC`

const shippingOfficesQuery = `SELECT Kode_Kantor, Nama_Kantor
FROM cch_kantor
ORDER BY Kode_Kantor ASC`

func (repository *MySQLComplaintRepository) ListActiveComplaintTypes(ctx context.Context) ([]complaints.ComplaintTypeOption, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, activeComplaintTypesQuery)
	if err != nil {
		return nil, fmt.Errorf("list active complaint types: %w", err)
	}
	defer rows.Close()
	options := make([]complaints.ComplaintTypeOption, 0)
	for rows.Next() {
		var option complaints.ComplaintTypeOption
		var description sql.NullString
		if err := rows.Scan(&option.Code, &description); err != nil {
			return nil, fmt.Errorf("scan active complaint type: %w", err)
		}
		if description.Valid {
			option.Description = description.String
		}
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read active complaint types: %w", err)
	}
	return options, nil
}

func (repository *MySQLComplaintRepository) ListActiveComplaintSources(ctx context.Context) ([]complaints.ComplaintSourceOption, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, activeComplaintSourcesQuery)
	if err != nil {
		return nil, fmt.Errorf("list active complaint sources: %w", err)
	}
	defer rows.Close()
	options := make([]complaints.ComplaintSourceOption, 0)
	for rows.Next() {
		var option complaints.ComplaintSourceOption
		if err := rows.Scan(&option.ID, &option.Name); err != nil {
			return nil, fmt.Errorf("scan active complaint source: %w", err)
		}
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read active complaint sources: %w", err)
	}
	return options, nil
}

func (repository *MySQLComplaintRepository) ListDestinationOffices(ctx context.Context, originOfficeCode string) ([]complaints.OfficeOption, error) {
	return repository.listOffices(ctx, "list eligible destination offices", destinationOfficesQuery, originOfficeCode)
}

func (repository *MySQLComplaintRepository) ListCopyOffices(ctx context.Context, originOfficeCode string) ([]complaints.OfficeOption, error) {
	return repository.listOffices(ctx, "list eligible copy offices", copyOfficesQuery, originOfficeCode)
}

func (repository *MySQLComplaintRepository) ListShippingServices(ctx context.Context) ([]complaints.ShippingServiceOption, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, shippingServicesQuery)
	if err != nil {
		return nil, fmt.Errorf("list shipping services: %w", err)
	}
	defer rows.Close()
	options := make([]complaints.ShippingServiceOption, 0)
	for rows.Next() {
		var option complaints.ShippingServiceOption
		if err := rows.Scan(&option.Code, &option.Description); err != nil {
			return nil, fmt.Errorf("scan shipping service: %w", err)
		}
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read shipping services: %w", err)
	}
	return options, nil
}

func (repository *MySQLComplaintRepository) ListShippingOffices(ctx context.Context) ([]complaints.OfficeOption, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, shippingOfficesQuery)
	if err != nil {
		return nil, fmt.Errorf("list shipping offices: %w", err)
	}
	defer rows.Close()
	options := make([]complaints.OfficeOption, 0)
	for rows.Next() {
		var option complaints.OfficeOption
		if err := rows.Scan(&option.Code, &option.Name); err != nil {
			return nil, fmt.Errorf("scan shipping office: %w", err)
		}
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read shipping offices: %w", err)
	}
	return options, nil
}

func (repository *MySQLComplaintRepository) listOffices(ctx context.Context, operation string, query string, originOfficeCode string) ([]complaints.OfficeOption, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, query, originOfficeCode)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}
	defer rows.Close()
	options := make([]complaints.OfficeOption, 0)
	for rows.Next() {
		var option complaints.OfficeOption
		if err := rows.Scan(&option.Code, &option.Name); err != nil {
			return nil, fmt.Errorf("scan %s: %w", operation, err)
		}
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", operation, err)
	}
	return options, nil
}

var _ complaints.LookupRepository = (*MySQLComplaintRepository)(nil)
