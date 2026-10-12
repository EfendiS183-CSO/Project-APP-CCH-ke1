package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/complaints"
)

func TestMariaDBDashboardCountsAndStatusTransitionConcurrency(t *testing.T) {
	database := openIsolatedMariaDB(t)
	createMariaDBIntegrationSchema(t, database)
	seedMariaDBDashboardFixtures(t, database)
	seedMariaDBStatusFixtures(t, database)

	t.Run("dashboard predicates", func(t *testing.T) {
		from := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, time.September, 30, 23, 59, 59, 999_999_000, time.UTC)
		query := complaints.OpenCountQuery{From: &from, To: &to}
		repository := NewMySQLComplaintRepository(database)
		counts := []struct {
			name string
			call func() (int64, error)
			want int64
		}{
			{name: "combined open", call: func() (int64, error) {
				return repository.CountOpenForCallCenter(context.Background(), query, "40005CC")
			}, want: 2},
			{name: "received open exact target", call: func() (int64, error) {
				return repository.CountReceivedOpenForCallCenter(context.Background(), query, "40005CC")
			}, want: 1},
			{name: "sent open origin only", call: func() (int64, error) {
				return repository.CountSentOpenForCallCenter(context.Background(), query, "40005CC")
			}, want: 1},
			{name: "completed dashboard", call: func() (int64, error) {
				return repository.CountCompletedForCallCenter(context.Background(), query, "40005CC")
			}, want: 1},
		}
		for _, test := range counts {
			t.Run(test.name, func(t *testing.T) {
				count, err := test.call()
				if err != nil || count != test.want {
					t.Fatalf("count = %d, error = %v, want %d", count, err, test.want)
				}
			})
		}
		lists := []struct {
			name        string
			officeMatch complaints.ComplaintOfficeMatch
			wantID      string
		}{
			{name: "received open exact target", officeMatch: complaints.MatchAllTargetsOnly, wantID: "TKT-RECEIVED"},
			{name: "sent open origin only", officeMatch: complaints.MatchOriginOnly, wantID: "TKT-SENT"},
		}
		for _, test := range lists {
			t.Run(test.name+" list", func(t *testing.T) {
				rows, err := repository.ListInvolvingOffice(context.Background(), complaints.ComplaintListQuery{
					OfficeCode: "40005CC", OfficeMatch: test.officeMatch, OpenOnly: true,
					From: &from, To: &to, Limit: complaints.DefaultComplaintPageSize,
				})
				if err != nil || len(rows) != 1 || rows[0].ComplaintID != test.wantID {
					t.Fatalf("report rows = %#v, error = %v, want only %s", rows, err, test.wantID)
				}
			})
		}
		t.Run("received-open legacy detail report", func(t *testing.T) {
			rows, err := repository.ListReceivedOpenReport(context.Background(), complaints.ReceivedOpenReportQuery{
				From: &from, To: &to, Limit: complaints.DefaultComplaintPageSize,
			}, "40005CC")
			if err != nil || len(rows) != 1 || rows[0].ComplaintID != "TKT-RECEIVED" || rows[0].LatestStatusCode != "102" {
				t.Fatalf("received-open report = %#v, error = %v", rows, err)
			}
		})
		t.Run("sent-open legacy detail report", func(t *testing.T) {
			rows, err := repository.ListSentOpenReport(context.Background(), complaints.SentOpenReportQuery{
				From: &from, To: &to, Limit: complaints.DefaultComplaintPageSize,
			}, "40005CC")
			if err != nil || len(rows) != 1 || rows[0].ComplaintID != "TKT-SENT" || rows[0].LatestStatusCode != "102" || rows[0].OriginOfficeUpdateCode != "40005CC" {
				t.Fatalf("sent-open report = %#v, error = %v", rows, err)
			}
		})
		t.Run("received-open duration day boundaries", func(t *testing.T) {
			var databaseToday time.Time
			if err := database.QueryRow("SELECT CURRENT_DATE").Scan(&databaseToday); err != nil {
				t.Fatalf("read MariaDB current date: %v", err)
			}
			for ageDays := 0; ageDays <= 4; ageDays++ {
				complaintID := fmt.Sprintf("TKT-DURATION-%d", ageDays)
				createdAt := databaseToday.AddDate(0, 0, -ageDays).Add(12 * time.Hour)
				if _, err := database.Exec(`INSERT INTO cchentri
					(ID_Pengaduan, Status_Akhir, Kantor_Asal, Kantor_Tujuan, Semua_Tujuan, Tanggal_Pengaduan, Tanggal_Tambah, Jenis_Kiriman)
					VALUES (?, '102', '40006CC', '40005CC', '40005CC', ?, ?, 'AGE')`, complaintID, createdAt, createdAt); err != nil {
					t.Fatalf("insert age-bucket complaint fixture: %v", err)
				}
				if _, err := database.Exec(`INSERT INTO cchentridiet
					(ID_Pengaduan, ID_Histori, Status_Update, Kantor_Asal_Update, Kantor_Tujuan_Update)
					VALUES (?, ?, '101', '40006CC', '40005CC')`, complaintID, "AGE-INIT"); err != nil {
					t.Fatalf("insert age-bucket distribution fixture: %v", err)
				}
			}
			from := databaseToday.AddDate(0, 0, -6)
			to := databaseToday.AddDate(0, 0, 1).Add(-time.Nanosecond)
			buckets, err := repository.CountReceivedOpenDurationBuckets(context.Background(), complaints.ReceivedOpenDurationQuery{
				From: &from, To: &to, ProductCode: "AGE",
			}, "40005CC")
			want := complaints.ReceivedOpenDurationBuckets{DayZero: 1, DayOne: 1, DayTwo: 1, DayThree: 1, DayFourPlus: 1}
			if err != nil || buckets != want {
				t.Fatalf("duration buckets = %#v, error=%v, want %#v", buckets, err, want)
			}
		})
	})

	t.Run("different idempotency keys serialize on expected status", func(t *testing.T) {
		command := validStatusTransitionCommand()
		command.ComplaintID = "TKT-RACE"
		command.IdempotencyKey = "race-request-key-0001"
		competing := command
		competing.IdempotencyKey = "race-request-key-0002"
		results := runConcurrentStatusTransitions(t, database, command, competing)
		successes := 0
		conflicts := 0
		for _, result := range results {
			switch {
			case result.err == nil && !result.value.Replayed:
				successes++
			case errors.Is(result.err, complaints.ErrStatusTransitionConflict):
				conflicts++
			default:
				t.Fatalf("unexpected concurrent transition result: %#v", result)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("successes/conflicts = %d/%d, want 1/1", successes, conflicts)
		}
		assertMariaDBTransitionState(t, database, command.ComplaintID, 1)
	})

	t.Run("same idempotency key replays one commit", func(t *testing.T) {
		command := validStatusTransitionCommand()
		command.ComplaintID = "TKT-REPLAY"
		command.IdempotencyKey = "replay-request-key-0001"
		results := runConcurrentStatusTransitions(t, database, command, command)
		successes := 0
		replays := 0
		for _, result := range results {
			if result.err != nil {
				t.Fatalf("concurrent exact retry failed: %v", result.err)
			}
			if result.value.Replayed {
				replays++
			} else {
				successes++
			}
			if result.value.StatusCode != command.StatusCode || result.value.ComplaintID != command.ComplaintID {
				t.Fatalf("unexpected transition result: %#v", result.value)
			}
		}
		if successes != 1 || replays != 1 {
			t.Fatalf("successes/replays = %d/%d, want 1/1", successes, replays)
		}
		assertMariaDBTransitionState(t, database, command.ComplaintID, 1)
	})
}

type concurrentStatusResult struct {
	value complaints.StatusTransitionResult
	err   error
}

func runConcurrentStatusTransitions(t *testing.T, database *sql.DB, commands ...complaints.StatusTransitionCommand) []concurrentStatusResult {
	t.Helper()
	start := make(chan struct{})
	results := make(chan concurrentStatusResult, len(commands))
	var workers sync.WaitGroup
	for _, command := range commands {
		command := command
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			value, err := NewMySQLStatusTransitionRepository(database).ApplyCallCenterStatusTransition(ctx, command)
			results <- concurrentStatusResult{value: value, err: err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	completed := make([]concurrentStatusResult, 0, len(commands))
	for result := range results {
		completed = append(completed, result)
	}
	return completed
}

func openIsolatedMariaDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("CCH_TEST_MARIADB_DSN"))
	if dsn == "" {
		t.Skip("CCH_TEST_MARIADB_DSN is not configured")
	}
	settings, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse CCH_TEST_MARIADB_DSN: %v", err)
	}
	if !strings.HasPrefix(strings.ToLower(settings.DBName), "cch_be_test_") {
		t.Fatalf("refusing integration test database without cch_be_test_ prefix")
	}
	settings.ParseTime = true
	settings.Loc = time.UTC
	database, err := sql.Open("mysql", settings.FormatDSN())
	if err != nil {
		t.Fatalf("open isolated MariaDB test database: %v", err)
	}
	database.SetMaxOpenConns(4)
	database.SetMaxIdleConns(4)
	t.Cleanup(func() { _ = database.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("ping isolated MariaDB test database: %v", err)
	}
	var objectCount int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()`).Scan(&objectCount); err != nil {
		t.Fatalf("inspect isolated MariaDB test schema: %v", err)
	}
	if objectCount != 0 {
		t.Fatalf("refusing non-empty MariaDB test schema (found %d tables/views)", objectCount)
	}
	return database
}

func createMariaDBIntegrationSchema(t *testing.T, database *sql.DB) {
	t.Helper()
	t.Cleanup(func() {
		for _, statement := range []string{
			"DROP VIEW IF EXISTS cchselesai",
			"DROP TABLE IF EXISTS cch_be_status_transition_idempotency",
			"DROP TABLE IF EXISTS cchentridet",
			"DROP TABLE IF EXISTS cchentrimas",
			"DROP TABLE IF EXISTS cch_jenis_penanganan",
			"DROP TABLE IF EXISTS cch_kantor",
			"DROP TABLE IF EXISTS cchentri",
		} {
			if _, err := database.Exec(statement); err != nil {
				t.Errorf("clean isolated MariaDB test schema: %v", err)
			}
		}
	})
	for _, statement := range []string{
		`CREATE TABLE cchentri (
			ID_Pengaduan VARCHAR(25) NOT NULL PRIMARY KEY,
			Status_Akhir CHAR(3) NOT NULL,
			Kantor_Asal VARCHAR(7) NOT NULL,
			Kantor_Tujuan VARCHAR(7) NOT NULL DEFAULT '',
			Kantor_Tembusan VARCHAR(100) NULL,
			Semua_Tujuan VARCHAR(255) NULL,
			Tanggal_Pengaduan DATETIME NULL,
			Tanggal_Tambah DATETIME NULL,
			Jenis_Kiriman VARCHAR(20) NULL,
			Nomor_Kiriman VARCHAR(100) NULL,
			Kantor_Status VARCHAR(7) NOT NULL DEFAULT '',
			Tanggal_Status DATETIME NULL,
			Request_Status CHAR(1) NOT NULL DEFAULT '0'
		) ENGINE=InnoDB`,
		`CREATE TABLE cch_jenis_penanganan (
			ID_Status CHAR(3) NOT NULL PRIMARY KEY,
			Aktif CHAR(1) NOT NULL
		) ENGINE=InnoDB`,
		`CREATE TABLE cch_kantor (
			Kode_Kantor VARCHAR(7) NOT NULL PRIMARY KEY,
			Regional VARCHAR(7) NULL
		) ENGINE=InnoDB`,
		`CREATE TABLE cchentrimas (
			ID_Pengaduan VARCHAR(25) NOT NULL,
			ID_Histori VARCHAR(25) NOT NULL,
			Status_Update CHAR(3) NOT NULL,
			Kantor_Asal_Update VARCHAR(7) NOT NULL,
			Kantor_Tujuan_Update VARCHAR(255) NOT NULL DEFAULT '',
			Tanggal_Update DATETIME(6) NOT NULL,
			Deskripsi_Update TEXT NOT NULL,
			Petugas_Update VARCHAR(20) NOT NULL,
			Durasi_Antar_Status BIGINT NULL,
			PRIMARY KEY (ID_Pengaduan, ID_Histori, Status_Update, Kantor_Asal_Update)
		) ENGINE=InnoDB`,
		`CREATE TABLE cchentridet (
			ID_Pengaduan VARCHAR(25) NOT NULL,
			ID_Histori VARCHAR(25) NOT NULL,
			Status_Update CHAR(3) NOT NULL,
			Kantor_Asal_Update VARCHAR(7) NOT NULL,
			Kantor_Tujuan_Update VARCHAR(7) NOT NULL,
			PRIMARY KEY (ID_Pengaduan, ID_Histori, Status_Update, Kantor_Asal_Update, Kantor_Tujuan_Update)
		) ENGINE=InnoDB`,
		`CREATE VIEW cchselesai AS SELECT * FROM cchentri WHERE Status_Akhir = '109'`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("create isolated MariaDB integration fixture: %v", err)
		}
	}
	migrationPath := filepath.Join("..", "..", "..", "..", "migrations", "0001_create_status_transition_idempotency.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read status-transition migration: %v", err)
	}
	if _, err := database.Exec(string(migration)); err != nil {
		t.Fatalf("apply status-transition migration to isolated test schema: %v", err)
	}
}

func seedMariaDBDashboardFixtures(t *testing.T, database *sql.DB) {
	t.Helper()
	fixtures := []struct {
		id, status, origin, targets string
		date                        time.Time
	}{
		{"TKT-RECEIVED", "102", "40006CC", "40005CC,40007CC", time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC)},
		{"TKT-SENT", "102", "40005CC", "", time.Date(2026, time.September, 16, 12, 0, 0, 0, time.UTC)},
		{"TKT-COMPLETED", "109", "40005CC", "40005CC", time.Date(2026, time.September, 17, 12, 0, 0, 0, time.UTC)},
		{"TKT-PARTIAL", "102", "40007CC", "140005CC9", time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)},
		{"TKT-OUTSIDE-DATE", "102", "40007CC", "40005CC", time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC)},
	}
	for _, fixture := range fixtures {
		if _, err := database.Exec(`INSERT INTO cchentri
			(ID_Pengaduan, Status_Akhir, Kantor_Asal, Kantor_Tujuan, Semua_Tujuan, Tanggal_Pengaduan, Tanggal_Tambah, Jenis_Kiriman, Nomor_Kiriman)
			VALUES (?, ?, ?, '', ?, ?, ?, 'REG', ?)`, fixture.id, fixture.status, fixture.origin, fixture.targets, fixture.date, fixture.date, "RESI-"+fixture.id); err != nil {
			t.Fatalf("insert dashboard fixture: %v", err)
		}
	}
}

func seedMariaDBStatusFixtures(t *testing.T, database *sql.DB) {
	t.Helper()
	for _, office := range []struct{ code, regional string }{
		{"40005CC", "40005"}, {"40006CC", "40005"}, {"40007CC", "40005"},
	} {
		if _, err := database.Exec(`INSERT INTO cch_kantor (Kode_Kantor, Regional) VALUES (?, ?)`, office.code, office.regional); err != nil {
			t.Fatalf("insert office fixture: %v", err)
		}
	}
	if _, err := database.Exec(`INSERT INTO cch_jenis_penanganan (ID_Status, Aktif) VALUES ('103', 'Y')`); err != nil {
		t.Fatalf("insert status fixture: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO cchentridet
		(ID_Pengaduan, ID_Histori, Status_Update, Kantor_Asal_Update, Kantor_Tujuan_Update)
		VALUES ('TKT-RECEIVED', 'H-INITIAL', '101', '40006CC', '40005CC')`); err != nil {
		t.Fatalf("insert received distribution fixture: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO cchentridet
		(ID_Pengaduan, ID_Histori, Status_Update, Kantor_Asal_Update, Kantor_Tujuan_Update)
		VALUES ('TKT-SENT', 'H-INITIAL', '101', '40005CC', '40006CC')`); err != nil {
		t.Fatalf("insert sent distribution fixture: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO cchentrimas
		(ID_Pengaduan, ID_Histori, Status_Update, Kantor_Asal_Update, Tanggal_Update, Deskripsi_Update, Petugas_Update)
		VALUES ('TKT-RECEIVED', 'H-INITIAL', '101', '40006CC', '2026-09-15 12:00:00', 'Initial state', 'fixture')`); err != nil {
		t.Fatalf("insert initial received history fixture: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO cchentrimas
		(ID_Pengaduan, ID_Histori, Status_Update, Kantor_Asal_Update, Tanggal_Update, Deskripsi_Update, Petugas_Update)
		VALUES ('TKT-RECEIVED', 'H-LATEST', '102', '40006CC', '2026-09-16 09:30:00', 'Latest state', 'fixture')`); err != nil {
		t.Fatalf("insert latest received history fixture: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO cchentrimas
		(ID_Pengaduan, ID_Histori, Status_Update, Kantor_Asal_Update, Tanggal_Update, Deskripsi_Update, Petugas_Update)
		VALUES ('TKT-SENT', 'H-INITIAL', '101', '40005CC', '2026-09-16 12:00:00', 'Initial state', 'fixture')`); err != nil {
		t.Fatalf("insert initial sent history fixture: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO cchentrimas
		(ID_Pengaduan, ID_Histori, Status_Update, Kantor_Asal_Update, Tanggal_Update, Deskripsi_Update, Petugas_Update)
		VALUES ('TKT-SENT', 'H-LATEST', '102', '40006CC', '2026-09-17 10:00:00', 'Latest state', 'fixture')`); err != nil {
		t.Fatalf("insert latest sent history fixture: %v", err)
	}
	for _, complaintID := range []string{"TKT-RACE", "TKT-REPLAY"} {
		if _, err := database.Exec(`INSERT INTO cchentri
			(ID_Pengaduan, Status_Akhir, Kantor_Asal, Kantor_Tujuan, Kantor_Tembusan, Semua_Tujuan)
			VALUES (?, '102', '40005CC', '40006CC', '40007CC', '40006CC,40007CC')`, complaintID); err != nil {
			t.Fatalf("insert status-transition parent fixture: %v", err)
		}
		previousAt := time.Now().UTC().Add(-time.Hour)
		previousID := previousAt.Format("20060102150405") + fmt.Sprintf("%02d", previousAt.Nanosecond()/10_000_000)
		if _, err := database.Exec(`INSERT INTO cchentrimas
			(ID_Pengaduan, ID_Histori, Status_Update, Kantor_Asal_Update, Tanggal_Update, Deskripsi_Update, Petugas_Update)
			VALUES (?, ?, '102', '40005CC', ?, 'Initial state', 'fixture')`, complaintID, previousID, previousAt); err != nil {
			t.Fatalf("insert initial status-history fixture: %v", err)
		}
	}
}

func assertMariaDBTransitionState(t *testing.T, database *sql.DB, complaintID string, expectedNewHistory int64) {
	t.Helper()
	var status string
	if err := database.QueryRow(`SELECT Status_Akhir FROM cchentri WHERE ID_Pengaduan = ?`, complaintID).Scan(&status); err != nil {
		t.Fatalf("read status-transition parent: %v", err)
	}
	if status != "103" {
		t.Fatalf("parent status = %s, want 103", status)
	}
	var historyCount int64
	if err := database.QueryRow(`SELECT COUNT(*) FROM cchentrimas WHERE ID_Pengaduan = ?`, complaintID).Scan(&historyCount); err != nil {
		t.Fatalf("count status history rows: %v", err)
	}
	if historyCount != expectedNewHistory+1 {
		t.Fatalf("history rows = %d, want %d including initial state", historyCount, expectedNewHistory+1)
	}
	var idempotencyCount int64
	if err := database.QueryRow(`SELECT COUNT(*) FROM cch_be_status_transition_idempotency WHERE complaint_id = ?`, complaintID).Scan(&idempotencyCount); err != nil {
		t.Fatalf("count status idempotency rows: %v", err)
	}
	if idempotencyCount != 1 {
		t.Fatalf("idempotency rows = %d, want exactly one committed request", idempotencyCount)
	}
}
