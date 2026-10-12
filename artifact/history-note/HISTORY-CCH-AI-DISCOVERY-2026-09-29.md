# Riwayat Sesi Analisa CCH dan Rencana AI

**Tanggal sesi:** 2026-09-29  
**Workspace:** `c:\laragon\www\cch`  
**Tujuan:** Menyimpan jejak pembelajaran CCH, analisa chatbot/AI, keputusan yang belum dibuat, gap, dan cara melanjutkan pekerjaan.

> **Register status resmi:** untuk daftar "sudah/belum" terkini di backend Go, baca `STATUS-CCH-BE.md` (root workspace). Dokumen ini adalah riwayat naratif; register itu adalah sumber kebenaran.

> Ini adalah riwayat terstruktur dari percakapan sesi, bukan ekspor verbatim chat. Nilai credential, secret, dan data pelanggan sengaja tidak disalin.

## Konteks dan Prinsip

- Pengguna ingin memahami CCH terlebih dahulu sebelum menentukan tech stack dan mengembangkan integrasi AI.
- AI architecture masih eksplorasi. Belum ada keputusan provider/model, local-vs-cloud, bahasa service, microservices, channel baru, atau otomasi routing.
- Perubahan ke depan perlu ditautkan ke sumber requirement, pemilik perilaku di kode, dan tes penerimaan yang membuktikan hasilnya.
- Backend CCH tetap menjadi otoritas untuk permission, validasi master, lifecycle tiket, histori, distribusi kantor, dan penyimpanan. Model AI bukan pemilik data bisnis.

## Urutan Pembelajaran

### 1. Baseline proyek

SRS Enterprise dan panduan AI dibandingkan dengan implementasi. Baseline yang ditemukan adalah PHP Maker 2023, Slim 4, Doctrine DBAL, PHP-DI, dan MySQL. SRS menggambarkan target migrasi/enterprise yang lebih luas (antara lain microservices, PostgreSQL, Kafka, dan frontend modern); komponen target itu tidak boleh diasumsikan sudah berjalan.

### 2. Lifecycle tiket CCH

Objek dan makna utama yang ditelusuri:

- `cchentri`: tiket utama dan snapshot status/kantor/waktu status terakhir.
- `cchentrimas`: histori perubahan status.
- `cchentridet`: distribusi kantor pada histori status.
- `cchmas`: ringkasan kasus.
- `cchfu`: rencana dan realisasi follow-up.
- `cchselesai`: view tiket dengan status akhir 109, bukan tabel arsip tiket baru.

Status yang teridentifikasi: 101 ENTRI, 102 RESPON, 103 KONFIRMASI, 104 RESOLVED, 109 SELESAI. Penambahan histori memperbarui snapshot di tiket utama dan menghitung durasi status. Hak akses terdiri dari permission tabel dan pembatasan data berdasarkan kantor/role.

### 3. Satu request dari route ke database/view

Use case form tambah pengaduan ditelusuri melalui:

`/cchentriadd` route -> `PermissionMiddleware` -> `CchentriController::add()` -> `ControllerBase::runPage()` -> page class `CchentriAdd` -> `Cchentri` model -> MySQL -> PhpRenderer/view.

Form menggunakan konfirmasi dua tahap. Model mengisi status awal 101 dan metadata sistem; setelah insert, hook membuat histori awal, distribusi kantor, dan ringkasan kasus, lalu mengarahkan pengguna ke alur dokumen.

### 4. Chatbot sebagai use case

Dua jalur dibedakan:

- Tanya-jawab umum melalui `api/chatbot/message.php`, AiRouter, dan provider AI.
- Intake keluhan terstruktur melalui `api/chatbot/intake.php` dan `CchChatbotIntakeWorkflow`.

Intake mengumpulkan field berurutan, mengambil pilihan dari master CCH, merangkum uraian, menampilkan hasil untuk diperiksa, meminta consent, menyimpan draft, kemudian membuat tiket lewat API CCH. Status draft mencakup `awaiting_confirmation`, `creating`, `pending_cs`, dan `created`; tabel draft juga mendefinisikan `collecting` dan `cancelled`.

CS reviewer harus role Call Center pada kantor intake, punya permission melihat profil dan menambah tiket, memilih profil, serta memberi attestation verifikasi identitas. API client menggunakan allowlist field. Setelah hasil API tidak pasti, workflow mencari tiket yang mungkin telah commit untuk mencegah retry duplikat.

### 5. AI architecture masih dipelajari

Kemampuan aktif yang terlihat: chat bebas dan ringkasan uraian keluhan dengan fallback. Pemeriksaan grounding ringkasan berupa overlap kata, bukan jaminan akurasi makna. Tidak ditemukan classifier sentimen atau auto-routing tiket aktif pada source aplikasi. Tidak ditemukan adapter Track & Trace/IPOS yang dipakai chatbot.

`AiRouter` memilih/routing provider dan tier AI; ini berbeda dari auto-routing tiket ke kategori/kantor. `LocalAiService` tidak diaktifkan; route Tier 2 memakai `CloudAiService` dan konfigurasi yang diperiksa menunjuk Mistral Direct cloud.

Opsi yang dicatat, belum dipilih:

- AI service PHP terbatas di backend CCH yang ada.
- AI service terpisah dengan API internal jika kebutuhan isolasi/scale membenarkan biaya operasionalnya.
- Gateway provider sebagai abstraksi provider, yang tidak otomatis memberi privasi lokal, akurasi, atau keamanan.

Tahapan eksperimen kandidat: discovery dan data classification -> bantuan ringkasan yang diperiksa manusia -> shadow classification -> saran dengan human approval -> otomasi terbatas setelah metrik/guardrail disetujui -> channel baru setelah keamanan/operasi siap.

### 6. Traceability

Contoh matriks kebutuhan -> kode pemilik -> tes penerimaan dimasukkan ke `LEMBAR-CATATAN-ANALISA.MD`. Contoh requirement ID mencakup `POL-INTAKE-01`, `POL-INTAKE-02`, `CCH-INTAKE-03`, `SRS-UC001`, `AI-SUMMARY-01`, dan `CCH-TICKET-01`.

Sumber harus diberi label: kebijakan produk yang dibahas pengguna, requirement SRS, baseline kode, atau temuan analisa. Jika konflik, jangan menyatakan selesai sebelum pemilik requirement memutuskan.

## Keputusan/Kebijakan yang Dibahas

- Kebijakan intake yang dituju: setelah consent, profil yang dapat dicocokkan secara unik dapat dipakai untuk auto-create tiket; profil yang tidak dapat dicocokkan harus masuk draft CS. Kecocokan field bukan bukti kepemilikan nomor/identitas.
- Implementasi saat dianalisa berbeda: jika profil cocok tepat satu, pakai profil; jika hasil ambigu (>1), tahan untuk CS; jika tidak ada hasil (0), kode membuat profil baru dan melanjutkan tiket. Gap ini sudah dicatat.
- Arsitektur AI, provider, model, dan tahapan otomasi belum diputuskan. Kerangka bertahap hanya bahan evaluasi.
- Endpoint intake saat dianalisa aktif hanya pada development dan localhost; endpoint chat umum menonaktifkan autentikasi. Belum siap dibuka ke pelanggan production.

## Temuan dan Gap yang Dicatat

### Keamanan dan kesiapan deployment

- `src/config.development.php`, `src/config.production.php`, dan Dockerfile terkonfirmasi tracked Git; konfigurasi berisi credential statis. Nilainya tidak disalin ke riwayat ini.
- Endpoint debug/test/provider di `api/chatbot` tidak tracked dan tidak di-ignore. Root `.htaccess` hanya membatasi `.env.ai_intake`; file PHP yang ada dapat dipanggil langsung bila deployment mengizinkannya.
- `docker-compose.yml` menggunakan build context root; Dockerfile menjalankan `COPY .`; tidak ditemukan `.dockerignore`. Endpoint lokal/untracked dan file bernama seperti key dapat masuk image jika image dibangun dari workspace ini.
- `message_mistral.php` memiliki API key hardcoded. `test_direct_9router.php` membaca `.env` serta dapat menampilkan prefiks credential/error. Tidak ada nilai key yang dicatat.
- Konfigurasi cookie menyetel `COOKIE_SECURE=false`; perlu override production HTTPS.
- Intake Tier 2 tidak terbukti lokal, meskipun namanya menyatakan on-premise.

### Intake, data, dan reliability

- Profil nol hasil cocok otomatis dibuat baru, padahal kebijakan produk mengarahkan profile-unmatched ke CS.
- Payload form dan jalur conversational tidak memiliki validasi backend setara: batas panjang/format email ada di `collectAnswer()`, tetapi `validatePayload()` untuk form tidak mengulang semua batas.
- Lookup jenis layanan mengambil pilihan aktif, tetapi validasi referensi tidak memeriksa `Status_Aktif`.
- Penghapusan percakapan mempertahankan draft `created` dan menolak `creating`, tetapi menghapus draft `pending_cs`. Lifecycle delete chat vs withdrawal submission perlu diputuskan.
- Collection sebelum consent berada di PHP session; kehilangan session dapat menghilangkan progress. Draft DB dibuat setelah consent dan save.
- `consent_at` disimpan; belum terlihat versi teks consent/channel yang dicatat.
- Workflow berada dalam namespace CCH dan memakai `new RuntimeException` tanpa import/qualify. Ini temuan statis; runtime probe belum berhasil karena quoting PowerShell, sehingga masih perlu tes kecil.
- API ticket dan tabel draft tidak satu transaksi. Conditional status update + reconciliation membantu, tetapi timeout/crash setelah ticket commit sebelum draft `created` masih perlu diuji.
- `scripts/test_chatbot_integration.php` memanggil provider AI nyata dan menulis row test ke database konfigurasi; tidak terlihat environment guard khusus.
- Tidak ditemukan suite intake/draft khusus untuk consent, validasi lintas-channel, matching profile, race/retry, delete, dan reconciliation. Editor validation dokumen bersih; tidak ada behavior test aplikasi yang dijalankan pada sesi ini.

### Kualitas AI dan fungsionalitas

- Ringkasan AI dapat jatuh ke cuplikan sumber; overlap kata bukan factuality verification.
- Chat API melabeli fallback AI sebagai sukses dan menyertakan metadata debug; intake client menampilkan raw body saat respons CCH non-JSON.
- Pesan dan jawaban disimpan di database/cache. Cache membaca berdasarkan expiry 24 jam; cleanup/retention policy belum tampak.
- Pelacakan paket di prompt/mock belum sama dengan koneksi ke data Track & Trace.
- SRS UC-001 menyebut WhatsApp, Track & Trace, Kafka, analisis sentimen/kategorisasi, dan nomor tiket konfirmasi. Implementasi yang ditelusuri belum memenuhi keseluruhan alur itu; target SRS perlu divalidasi sebagai baseline/fase.

## Dokumen Kerja

- [LEMBAR-CATATAN-ANALISA.MD](LEMBAR-CATATAN-ANALISA.MD): baseline, lifecycle, chatbot, pilihan yang belum diputuskan, traceability, dan temuan pembelajaran.
- [LEMBAR-CATATAN-REVISI.MD](LEMBAR-CATATAN-REVISI.MD): gap yang dapat ditindaklanjuti, risiko, prioritas, dan kriteria revisi.
- File ini adalah history terstruktur untuk mengingat konteks dan melanjutkan sesi. Ini bukan backup verbatim dari seluruh teks chat atau pengganti Git/database backup.

## Cara Melanjutkan

1. Baca bagian Traceability dan Temuan Pendalaman di lembar analisa ini.
2. Baca backlog prioritas di lembar revisi; credential yang mungkin terekspos perlu ditangani sebelum channel pelanggan dibuka.
3. Untuk setiap perubahan, pilih requirement ID, verifikasi pemilik kode, tulis acceptance tests positif/negatif, jalankan pada environment terisolasi, lalu catat bukti dan risiko tersisa.
4. Jangan mulai dari pemilihan model/arsitektur sebelum use case, klasifikasi data, owner keputusan, dan metrik keberhasilan disepakati.

## Kelanjutan Sesi - Backend Modular dan Database (2026-09-29)

Bagian ini melanjutkan history di atas. Ini ringkasan keputusan dan bukti pekerjaan, bukan ekspor verbatim percakapan. Secret, password, token, dan data pelanggan tidak dicatat.

### Keputusan Proyek yang Dikonfirmasi User

- Database tetap memakai schema `cch` yang sudah ada; PostgreSQL/migrasi database dikecualikan untuk saat ini.
- Saat konfigurasi aktif CCH development diperiksa, target DB menunjuk ke `127.0.0.1:3306`, schema `cch`. Listener Laragon menjawab sebagai **MariaDB 10.4.32**; client `mysql.exe` terpasang versi 8.0.30. Metadata menunjukkan 42 base tables dan 18 views pada saat pemeriksaan.
- Pemeriksaan database hanya membaca `INFORMATION_SCHEMA` dan keberadaan object inti. Tidak ada isi row tiket/pelanggan dibaca atau diubah pada pemeriksaan ini.
- Stack target: GoFiber v3 untuk BE, React JS untuk FE, Python/FastAPI untuk AI Engine. OOP dan pemisahan modular per tanggung jawab menjadi standar. PostgreSQL tidak dipilih.
- Fiber v3.5.0 memerlukan Go 1.25+. Go 1.24.5 sistem dipertahankan; Go toolchain otomatis yang lebih baru dipakai oleh modul saat test/run.
- Arsitektur harus modular, tidak menumpuk semua business domains dalam satu file/service. `cmd/api` saat ini hanya composition root pengembangan; deployment/domain boundaries final belum disetujui.

### Fondasi yang Sudah Dibuat

- `Project-APP-CCH-ke1/CCH-BE` menggunakan Fiber v3.5.0 dan `go-sql-driver/mysql` melalui `database/sql`.
- DB config dibaca dari `CCH_DB_HOST`, `CCH_DB_PORT` (default 3306), `CCH_DB_USER`, `CCH_DB_PASSWORD`, dan `CCH_DB_NAME`. Password wajib tidak kosong; tidak memakai `root` secara default.
- `OpenMySQL` membuat DSN melalui konfigurasi driver, memakai `utf8mb4`, `parseTime`, UTC, timeout, dan pool limit. Belum ada akun aplikasi DB yang dikonfigurasi, sehingga Go belum membuat koneksi, query, atau write ke DB.
- `GET /healthz` adalah liveness; `GET /readyz` melakukan ping dan mengembalikan status `not_configured`, `unavailable`, atau `connected`. Error DB mentah tidak dikirim ke client.
- API foundation menyediakan `/api/v1/`, request ID `X-Request-ID`, JSON error envelope, dan generic internal error response. Vite mem-proxy `/api/v1` ke Go.
- Dokumentasi batas modular ada di `Project-APP-CCH-ke1/CCH-BE/README.md`; alur domain CCH ada di `Project-APP-CCH-ke1/CCH-BE/docs/cch-domain-workflow.md`.

### Autentikasi dan ACL CCH yang Diverifikasi

- JWT lama memakai header `X-Authorization` dengan optional `Bearer` prefix, HS512, issuer dari `SERVER_NAME`, dan nested `security` claims: `username`, `userid`, `parentuserid`, `userlevelid`, `permission`.
- Konfigurasi CCH memetakan `userid` ke `cch_users.Office`, `parentuserid` ke `cch_users.Report_To`, dan `userlevelid` ke `cch_users.User_Level`.
- CCH reloads privilege levels dari `cch_userlevelpermissions`, ORs masks untuk multi-level user, lalu meng-intersect dengan token permission bila mask token > 0. Mask token 0 tidak memberi batas tambahan.
- `cch_userlevelpermissions.Table_Name` memakai gabungan `PROJECT_ID` GUID + table variable. System Administrator `-1` adalah special principal yang diverifikasi lewat konfigurasi terpisah; Go tidak boleh memberi admin access hanya dari claim `-1`.
- Go memiliki verifier HS512 dengan allowlist algorithm, issuer/issued-at/expiry checks, leeway 30 detik, key minimal 32 byte, dan input secret/issuer `CCH_JWT_SECRET`/`CCH_JWT_ISSUER`. Middleware menerima token dari `X-Authorization` header saja; query/cookie fallback lama sengaja tidak disalin.
- `internal/modules/authorization` mengimplementasikan bitmask/action dan tested mask composition. Ini pure policy, belum membaca ACL DB dan belum menegakkan office scope.
- Verifier/middleware **belum dipasang di business routes**. Tidak ada real legacy JWT atau credential yang dipakai. Signing key legacy ditemukan statis pada tracked config dan harus dianggap terekspos; jangan salin ke Go. Rotasi key dan penyimpanan secret terkoordinasi diperlukan sebelum interoperability.
- Status akun `Activated`/`Locked`, office saat ini, report-to, dan permission perlu direvalidasi server-side agar token lama tidak memberi scope stale. Belum ada account/permission repository di Go.

### Validasi dan Runtime

- `go test ./...`, `go vet ./...`, gofmt, serta editor diagnostics lulus setelah perubahan auth/ACL.
- API smoke test sebelumnya: `/api/v1/` 200; unknown route 404 dengan request ID pada header/body; `/healthz` 200; `/readyz` 503 `not_configured` tanpa DB env.
- React FE: ESLint dan Vite production build lulus; proxy `/api/v1/` mencapai Go. FE dev server sebelumnya tersedia pada `http://127.0.0.1:5173/`.
- Proses lokal Go/AI/FE mungkin berhenti ketika sesi berikutnya dimulai; cek status port/proses sebelum menjalankan ulang. Jangan mengasumsikan service masih hidup.
- Git status terakhir: seluruh `Project-APP-CCH-ke1/` untracked; history file ini juga untracked. Tidak ada stage atau commit yang dibuat.
- File tak dikenal `Project-APP-CCH-ke1/CCH-BE/internal/platform/config/file.jpe` ditemukan saat inventory; tidak berhasil diidentifikasi dan sengaja tidak diubah/dihapus.

### Cara Melanjutkan Berikutnya

1. Baca bagian ini, `Project-APP-CCH-ke1/CCH-BE/docs/cch-domain-workflow.md`, dan backlog revisi lama sebelum coding business endpoints.
2. Implementasikan account/permission repository read-only dengan query parameterized: ambil current `cch_users` status/office/report-to/level dan ACL untuk key `PROJECT_ID + table variable`. Jangan select field password.
3. Tambahkan tests untuk akun inactive/locked, office/token mismatch, multi-level permission OR, token mask, table mismatch, special admin denial, dan office scope. Gunakan fixture/mock atau DB test lokal terisolasi.
4. Jangan pasang middleware JWT pada endpoint tiket sampai rotasi key disetujui, secret issuer/verifier tersedia melalui env/secret store, serta account/office/permission revalidation lulus.
5. Setelah guardrails siap, mulai satu vertical slice read-only (misalnya complaint lookup/list yang office-scoped); baru kemudian desain create-ticket transaction/reconciliation mengikuti `cchentri` + `cchentrimas` + `cchentridet` + `cchmas`.
6. Jangan memakai root atau melakukan write ke schema `cch` dalam development/test normal. Tidak ada data migration dan tidak ada business row yang disentuh pada sesi ini.

## Kelanjutan Sesi 2026-09-30

- Follow-up read-only Call Center 2026-09-30: module baru `internal/modules/followups` menyediakan `Service.ListForCallCenterTicket`, SQL adapter atas `cchfual`, dan Fiber handler `GET /api/v1/complaints/call-center/:id/follow-ups` dalam opt-in registrar. Requires `View` cchentri + `List` cchfual, role tunggal 3, office DB-terverifikasi; parent harus open dan exact origin/`Semua_Tujuan` match. Follow-up `Kantor` sengaja tidak difilter agar record kantor terkait pada tiket visible dapat dilihat sesuai master/detail legacy; assigned staff disembunyikan. Empty follow-up pada parent valid mengembalikan `[]`, unknown/inaccessible parent menjadi 404. Read-only; add/edit/realize/global queue belum dibuat. Mock/service/handler/route composition tests lulus; no DB business query.

- Komposisi route opt-in 2026-09-30: `CCH_ENABLE_CALL_CENTER_COMPLAINT_READ_API` default false. `cmd/api` membangun access repository/service, complaint repository/service, JWT verifier, dan authenticated route registrar hanya saat flag true. Jika MySQL atau JWT config/verifier invalid, startup gagal; default tanpa flag tetap hanya health/version routes. `NewServer` mendukung registrar versi opsional dan menolak request tanpa JWT. Tes `cmd/api`, config, dan httpapi lulus. Flag bukan bukti key lama sudah dirotasi; operator harus mengaktifkan hanya setelah rotasi/secret provisioning terkoordinasi.

- Detail ticket read-only Call Center 2026-09-30: service memerlukan `View` pada `cchentri`, role tunggal 3, office DB-terverifikasi, parent status non-109, dan origin atau exact token `Semua_Tujuan`. Query hanya mengembalikan detail field yang legacy view tampilkan; `ID_Pelanggan` dan audit fields tidak disertakan. Unknown ID dan out-of-scope ID sama-sama menjadi 404. Handler tersedia di subpackage `complaints/handler` tetapi belum diregistrasikan pada `NewServer`; tests memakai sqlmock/service stub, tanpa akses DB nyata.

- Langkah 2 dilaksanakan: `CCH-BE/internal/modules/authorization/repository.MySQLAccessRepository` read-only dan memakai query parameterized. Repository membaca hanya Username, User_Level, Report_To, Office, Activated, Locked (tidak membaca Password), menolak akun tidak aktif/terkunci, mismatch office/report-to/role antara DB dan token, serta level System Administrator `-1` sampai mapping admin disetujui.
- ACL dibaca dengan key `PROJECT_ID + table variable`; mask role di-OR lalu dibatasi positive token mask. `sqlmock` menguji multi-role, mask token, status akun, claim stale, special-admin denial, key tabel proyek/tidak dikenal, dan ketiadaan kolom password pada query.
- Verifikasi lulus: `go test ./...`, `go vet ./...`, serta diagnostics editor. Tidak ada koneksi/query ke schema `cch`, route bisnis, atau perubahan data DB. Repository belum dipasang di middleware/composition root.
- `AccountAccess` mengembalikan office tepercaya, tetapi predicate office-scope masih wajib dibuat per use case sebelum endpoint data dibuat. Langkah berikutnya: tentukan policy office scope dari use case read-only pertama, lalu pasang repository melalui komposisi BE tanpa mengaktifkan JWT route bisnis sebelum prasyarat rotasi key/secret terpenuhi.

- Organisasi kode disepakati user 2026-09-30: repository mengikuti domain, bukan satu folder global. Struktur authorization memakai `internal/modules/authorization/permissions.go` dan `internal/modules/authorization/repository/mysql_access.go` beserta tesnya. Package repository boleh memakai policy package authorization; package induk tidak mengimpor subpackage repository agar tidak membentuk import cycle.

- Analisa lanjutan BE 2026-09-30: `Cchentri` list legacy melewati `PermissionMiddleware`; `recordsetSelecting()` mengembalikan kosong jika filter awal kosong (`0=101`). Role biasa memakai scope asal/tujuan/tembusan; Call Center/pusat/UPT memakai scope `Semua_Tujuan` atau asal hanya ketika filter memuat status terbuka. `IsAdmin()` melewati office filter dan dapat dipicu level/admin permission, jadi tidak aman disalin sebagai unrestricted access. Dashboard open, received, sent, dan closed memakai predicate berbeda. `Semua_Tujuan` berisi comma-delimited routing kantor; legacy `LIKE '%office%'` rawan partial match. Kesimpulan: jangan buat endpoint list generik dulu; policy harus disepakati per use case dan pakai exact token membership.
- Komponen baru `authorization.AccessService` memverifikasi action mask dari `AccountAccessLoader`, memvalidasi action/mask, dan mengembalikan scope akun tepercaya. MySQL adapter mengimplementasikan interface melalui compile-time assertion. Tes package authorization lulus; service belum wired ke HTTP route, dan belum ada query atau perubahan data DB.
- Pendalaman role khusus 2026-09-30: `CurrentUserLevel()` adalah active user-level ID; permission mask terpisah. `Cchentrimas` recordset hook yang diperiksa tidak punya office filter aktif, tetapi add-status menolak tiket 109 dan khusus Call Center membatasi tiket ke `Kantor_Asal` sendiri. Lookup status: origin office mendapat 103/109; non-origin UPT/UPT Bandara/Call Center/Admin UPT mendapat 102; Pusat/Admin Pusat 102/109; role lain dapat 109 bila office mereka regional kantor asal. `Cchselesai` punya filter role sendiri. `cchfual/cchfuhi` sengaja melonggarkan filter office untuk UPT/CC terkait, memberlakukan empty-result tanpa filter bagi sebagian role, dan Regional hanya menambahkan descendant-office scope pada query rentang tanggal. Ini UI/model behavior, bukan transition contract yang sudah disetujui.
- Keputusan sementara: pisahkan policy BE complaint visibility, status history, completed tickets, dan follow-up. Jangan gunakan satu `OfficeScope` generik atau membuka mutation/status endpoints sebelum multi-role/active-role behavior serta transisi status disepakati dan diuji.
- Authorization action middleware 2026-09-30: `internal/platform/httpapi.RequireTableAction` memakai `VerifiedIdentity` dari context, memanggil `authorization.AccessService`, memetakan stale account ke 401, permission/principal denial ke 403, konfigurasi action/table invalid ke 500, dan error repository ke 503 tanpa mengekspos detail. AccountAccess tepercaya tersedia untuk handler via context accessor. Tes HTTP mencakup allow, missing identity/service, stale claim, permission denial, dan backend failure. Middleware belum dipasang di business route; JWT key rotation tetap prasyarat. `go test ./...`, `go vet ./...`, diagnostics lulus.
- Komponen domain complaints 2026-09-30: `ComplaintRouting.InvolvesOffice` memeriksa keterkaitan kantor lewat `Kantor_Asal`, `Kantor_Tujuan`, `Kantor_Tembusan`, dan token exact comma-delimited `Semua_Tujuan`. `ComplaintListQuery` memvalidasi office/pagination (default 25, maksimum 100). `repository.MySQLComplaintRepository.ListInvolvingOffice` menjalankan query parameterized, memakai exact CSV token matching, deterministic order, dan projection minimal (tanpa nama/kontak/alamat/uraian pelanggan). Tes mock memastikan query/scan/pagination dan penolakan input invalid. Query ini hanya mencari asosiasi routing, bukan authorization/office visibility policy; belum ada handler atau route pengaduan. `go test ./...`, `go vet ./...`, dan diagnostics lulus. Sebelum endpoint, tentukan scope per use case karena list/dashboard/status/follow-up legacy berbeda.
- Vertical slice read-only Call Center 2026-09-30: `complaints.Service.ListOpenForCallCenter` memerlukan `authorization.AccessService` dengan `ActionList` atas `cchentri`, lalu mensyaratkan `UserLevelIDs` persis satu elemen `3`. Office selalu diambil dari `AccountAccess` DB-terverifikasi, bukan dari query pemanggil. Repository memakai predicate `Kantor_Asal = office OR exact token in Semua_Tujuan` dan `Status_Akhir <> '109'`, selaras dengan dashboard open-call-center legacy; tidak memberi akses ke completed tickets atau mutasi histori. Multi-role ditolak karena helper legacy `IsCallCenter()` membandingkan active level dengan nilai tunggal dan API belum memiliki active-role contract. SQL/service tests memakai mocks; route belum dibuat dan tidak ada akses DB nyata. `go test ./...`, `go vet ./...`, diagnostics lulus.
- HTTP handler Call Center 2026-09-30: `internal/modules/complaints/handler` menyediakan `GET /api/v1/complaints/call-center/open` bila nanti diregistrasikan. Handler mengambil identity dari JWT context, menerima pagination saja (office query diabaikan), mengembalikan projection JSON minimal dan pagination metadata, memetakan input invalid ke 400, role/permission denial ke 403, missing auth ke 401, dan backend error generik ke 503. Tes memakai `NewServer` dan verifier stub. Handler belum didaftarkan di `NewServer`; tidak ada route bisnis aktif atau query DB nyata. Full `go test ./...`, `go vet ./...`, dan diagnostics lulus.
- Timeline status read-only Call Center 2026-09-30: `Service.ListOpenTicketHistoryForCallCenter` memerlukan `List` pada `cchentri` dan `cchentrimas`, role tunggal 3, serta office/role identik pada dua revalidasi account; office request dibuang. Repository JOIN ke parent `cchentri`, membatasi `Status_Akhir <> '109'` dan origin atau exact token `Semua_Tujuan`, lalu mengurutkan `Tanggal_Update ASC, ID_Histori ASC`. Projection menampilkan field yang legacy list tampilkan, termasuk deskripsi update dan status label, tanpa data kontak pelanggan/dokumen. Handler `GET /api/v1/complaints/call-center/:id/history` memvalidasi ticket ID/pagination dan mengembalikan timeline JSON; handler belum didaftarkan ke `NewServer`. SQL/service/HTTP tests memakai mocks; tidak ada query DB nyata. Full test/vet/diagnostics lulus.
- Analisa lanjutan status-history 2026-09-30: `cchentrimas` adalah child history keyed `(ID_Pengaduan, ID_Histori, Status_Update, Kantor_Asal_Update)` dan `cchentridet` menyimpan satu kantor tujuan per key gabungan. Normal CCH list detail-add mengirim `showdetail=cchentrimas`; config `USE_TRANSACTION=true`, dan generated Add memulai transaksi bila current detail table tidak kosong. Pada jalur normal, insert distribusi dalam `rowInserting` dan row history tercakup transaksi. Generated Add commit/rollback terjadi sebelum `rowInserted`; hook setelah commit mengubah snapshot `cchentri` dan durasi, sehingga parent sync tidak atomic dengan history. Hook menolak status `109` dan khusus Call Center memverifikasi kantor asal; pilihan role/status lain terutama dibatasi oleh lookup UI, belum terlihat sebagai guard transisi penuh di row insert. API route khusus status tidak ditemukan di `src/apiroutes.php`. Sebelum mutasi Go, definisikan transition matrix server-side, idempotency, race/closed-ticket handling, atomic writes across three tables, dan perilaku dokumen yang saat ini menyusul lewat `cchentridok`.

### Kelanjutan BE 2026-09-30

- Update guest issuance 2026-09-30 (supersedes the earlier note below saying there was no route): `POST /api/v1/intake/sessions` now exists in `internal/modules/intake/session/handler`, registered only by `CCH_ENABLE_GUEST_INTAKE_SESSION_API=true`. Startup requires MySQL, a distinct guest signing key, issuer, explicit TTL, and exact origin allowlist. Handler applies 5 requests/IP/minute in-process and returns `Cache-Control: no-store`; it only issues a guest token, with no draft binding or ticket write.
- Follow-up office list read-only: endpoint `GET /api/v1/follow-ups/call-center/office` membaca `cchfu` dengan `Kantor =` office hasil revalidasi ACL, memerlukan role Call Center tunggal (`3`) dan `List` permission pada `cchfu`; office dari request tidak digunakan. Query parameterized, pagination default 25/max 100, projection terbatas. Dipilih karena cocok dengan `Cchfu::recordsetSelecting()`; queue `cchfuhi` tidak dibuat global karena legacy scope sengaja longgar untuk Call Center dan hasil kosong bila filter tidak diberikan.
- Tes service, MySQL adapter (`sqlmock`), HTTP handler, dan route composition ditambahkan. `go test ./...`, `go vet ./...`, dan editor diagnostics lulus. Tidak ada koneksi/query ke database CCH atau business write. Endpoint tetap di balik `CCH_ENABLE_CALL_CENTER_COMPLAINT_READ_API` (default false) dan JWT.
- BE belum tuntas: belum ada ticket create/status transition/follow-up writes, attachment API, master-data API, laporan, atau policy untuk role selain Call Center. Jangan membuat business writes sebelum aturan transisi status, atomicity parent/history/distribution, idempotency, office policy, dan key-rotation/secret gates disetujui. Langkah aman berikutnya adalah menetapkan serta men-trace requirement untuk master-data reads yang diperlukan UI; operasi tulis menunggu keputusan workflow dan acceptance tests.
- Complaint intake lookup read-only: `GET /api/v1/complaints/call-center/lookups` mengembalikan jenis pengaduan aktif, sumber aktif, kode kantor asal tepercaya, kantor tujuan/tembusan sesuai filter `lookupSelecting()` legacy, layanan kiriman dan kantor asal/tujuan kiriman sesuai lookup legacy (tanpa filter aktif tambahan). Memerlukan `Lookup` pada `cchentri` dan role Call Center tunggal; tidak mengembalikan profil customer atau menerima scope kantor dari request. Handler/repository/service dan tes route/HTTP/sqlmock mencakup validasi role/permission, office filter, dan mapping response. Feature flag read API tetap default false. Status tetap server-owned; customer matching serta API tulis tiket belum dibuat karena policy dan validasinya perlu dimodelkan.
- Customer profile search read-only kini menjadi domain `internal/modules/customers` dengan service, repository MySQL, dan controller/handler tersendiri; `cmd/api` hanya melakukan composition. Endpoint `GET /api/v1/complaints/call-center/customers/search?q=...` tetap membutuhkan `Lookup` pada `cchentricus` dan role Call Center tunggal. Query nama/HP prefix minimum 3 karakter, maksimum 20 hasil, exact office `Kantor_Tambah` dari akses akun DB-revalidated, SQL wildcard di-escape; output berisi nama/email/HP/alamat untuk mengisi snapshot tiket dan aman terhadap `Alamat=NULL`. HTTP/service/SQL tests mencakup auth, role, query pendek, office spoofing, wildcard, dan error generik. Ini bukan verifikasi identitas/nomor HP; policy unmatched/ambiguous tetap perlu human review. Endpoint read-only dan mengikuti feature flag default false.
- Follow-up due-today read-only: route `GET /api/v1/follow-ups/call-center/office/due-today` tetap dimiliki controller domain `followups`. Membaca view legacy `cchfuhi` (tanggal database hari ini dan belum terealisasi), membutuhkan `List` pada `cchfuhi` dan role Call Center tunggal; office selalu dari ACL DB-revalidated dengan exact filter dan pagination bounded. HTTP/service/sqlmock/route tests lulus; tidak ada write/query DB nyata. Route tetap di balik feature flag default false dan JWT.
- PII route gating: `CCH_ENABLE_CALL_CENTER_CUSTOMER_SEARCH_API` kini terpisah dari `CCH_ENABLE_CALL_CENTER_COMPLAINT_READ_API`. Kedua arah diuji: complaint flag saja tidak mendaftarkan customer search, customer flag saja tidak mendaftarkan complaint routes. Kedua registrar memerlukan DB/JWT saat flag masing-masing true; tests juga membuktikan default-off dan customer route 401 tanpa JWT bila khusus diaktifkan.
- Analisa create-ticket adapter 2026-09-30: `CchentriAdd` API legacy mengaktifkan transaksi untuk `IsApi()` dan memanggil `rowInserted()` sebelum commit, sehingga jalur `/api/add/cchentri` adalah pemilik side effects tiket/history/distribusi/ringkasan. Client PHP yang ada login dengan akun service dan office tetap; jangan menyalinnya ke Go karena audit petugas/office jadi salah. Go JWT middleware saat ini hanya menyimpan `VerifiedIdentity`, bukan token tervalidasi yang dibutuhkan untuk delegation ke CCH. Belum dibuat POST/write API. Sebelum implementasi: sepakati same-user token forwarding setelah rotasi key, allowlisted/TLS base URL CCH, pemetaan payload dan validasi master/profile, serta idempotency/reconciliation untuk timeout/retry. Jangan menulis langsung ke tabel legacy.
- Adapter transport CCH 2026-09-30: `internal/modules/complaints/gateway/legacycch` sekarang memiliki payload bertipe dan HTTPS-only `POST /api/add/cchentri` client; meneruskan token terverifikasi yang diberikan pemanggil, membatasi response 1 MiB, tidak mengikuti redirect, dan tidak memunculkan body/error upstream. `httpapi.VerifiedBearerTokenFromContext` hanya memberi token normalized setelah middleware berhasil memverifikasi signature. TLS mock tests membuktikan request/payload/header, redirect rejection, invalid response, dan PII/error-body non-leak. Adapter belum wired ke service/controller/route, tidak ada request CCH yang dikirim, dan tetap tidak memiliki durable idempotency; write API belum aman diaktifkan.
- Customer profile resolver internal: `customers.CallCenterSearchService.GetProfileForTicket` memvalidasi ID positif, memerlukan `Lookup` + role Call Center tunggal, mengambil office dari ACL DB-revalidated, lalu query `cchentricus` dengan exact `ID_Pelanggan` dan `Kantor_Tambah`. Profil tak ditemukan dan profil kantor lain dipetakan ke `ErrProfileNotFound`; snapshot berasal dari DB (alamat nullable ditangani). Ini belum route HTTP atau ticket write, dan hanya disiapkan sebagai input kanonis untuk use case create-ticket berikutnya.
- Verifikasi schema localhost 2026-09-30 (metadata saja melalui bootstrap konfigurasi development aktif, tanpa membaca row bisnis): schema `cch` berisi tabel `cchentri` dan `cch_chatbot_intake_drafts`. Draft memiliki unique `conversation_id`, `user_id`, state consent/creating/pending_cs/created, `consent_at`, dan `ticket_id`. Tidak ditemukan tabel/kolom idempotency atau correlation generik; unique conversation hanya cocok untuk chatbot flow, bukan otomatis untuk request create umum FE. Tidak ada DDL/write dilakukan.
- Guest intake identity sesuai pilihan user: Go menjadi pemilik token conversation guest. `internal/modules/intake/session` menerbitkan HS256 token terpisah dengan secret/issuer sendiri, audience `cch-intake`, purpose `cch_intake_session`, conversation ID acak 128-bit tanpa PII, TTL wajib eksplisit maksimum 24 jam. Controller `POST /api/v1/intake/sessions` hanya aktif dengan `CCH_ENABLE_GUEST_INTAKE_SESSION_API=true`, DB config, signing key berbeda dari JWT staf, exact Origin allowlist dan limiter in-process 5/IP/menit. Tes menolak origin lain, request setelah rate limit, token tampered/expired/wrong-purpose, dan memastikan verifier staf CCH HS512 menolak token intake. Route hanya menerbitkan token; belum ada ownership binding ke `cch_chatbot_intake_drafts`, edge-level rate limit, retention/revocation, maupun ticket workflow.

### Kelanjutan BE 2026-10-01 - Guest Intake Submission

- Submission domain dilanjutkan di `internal/modules/intake/submission`: request normalization/consent, validasi referensi aktif, exact profile matching, klaim draft unik per `conversation_id`, transisi `creating/created/pending_cs`, dan reconciliation exact-ticket setelah hasil upstream tidak pasti. Nol atau lebih dari satu profile match masuk `pending_cs`; tidak ada pembuatan profile otomatis.
- MySQL repository menggunakan parameterized query pada master CCH, `cchentricus`, `cchentri`, dan `cch_chatbot_intake_drafts`. Schema draft lokal memiliki unique `conversation_id`; Go tidak menulis langsung ke tabel tiket. Tabel ticket/history/distribution/summary dikelola API legacy `/api/add/cchentri` melalui adapter HTTPS.
- Controller domain terpisah: `POST /api/v1/intake/submissions`, header `X-Intake-Session`, JSON allowlist, consent wajib, exact Origin allowlist/CORS preflight, body maksimum 64 KiB, rate limit in-process 5/IP/menit, dan no-store response. `cmd/api` hanya melakukan composition.
- Route hanya didaftarkan jika `CCH_ENABLE_GUEST_INTAKE_SUBMISSION_API=true`; selain DB, mensyaratkan guest session config aktif, key intake berbeda dari staff JWT, staff JWT verifier valid, HTTPS CCH base URL, kredensial akun teknis, dan office asal tepercaya. Akun teknis diverifikasi terhadap role tunggal Call Center, office configured, dan `Add` permission `cchentri` sebelum ticket Add.
- Tes service, sqlmock repository, HTTP controller, dan route composition memakai doubles/mocks; tidak ada request CCH nyata atau row bisnis dibuat. Saat checkpoint ini dites fokus intake dan `cmd/api` lulus; full suite/vet final perlu dijalankan setelah dokumentasi/perapian.
- Keterbatasan: unique conversation claim dan exact-field reconciliation mencegah sebagian replay/duplikasi, tetapi bukan idempotency atomik yang dikelola legacy CCH. Hasil tak pasti yang tidak bisa dicocokkan atau ambigu ditahan untuk CS dan tidak di-retry otomatis. Retensi/penghapusan PII draft belum ditetapkan; route tetap default-off dan tidak siap production sebelum kebijakan retensi, edge rate limiting, key rotation, serta least privilege disetujui.

- Kelanjutan BE 2026-10-01 - CS review intake: domain `internal/modules/intake/review` kini memiliki GET queue, GET pending detail, GET profile candidate search, dan POST create-ticket; controller review dipisah pada domain intake. Gate `CCH_ENABLE_GUEST_INTAKE_REVIEW_API` default-off. Reviewer harus staff JWT valid, role Call Center tunggal, office sama dengan office intake, `View cchentricus`, dan `Add cchentri`; detail/search no-store, search min 3 chars/max 20 dan SQL wildcard di-escape.
- CS create memakai `identity_verified=true`, consent tersimpan, canonical name/email/phone/address dari profil, dan allowlist koreksi kantor tiga field. Query reference aktif; CCH technical account diverifikasi JWT/role/office/Add; conditional SQL claim `pending_cs -> creating` menyimpan corrected payload sebelum CCH Add. Exact-field reconciliation mengubah satu match jadi created, nol/ambigu ke pending CS. Tidak ada direct ticket-table write. Batas kolom alamat/uraian diselaraskan ke 5.000 chars legacy.
- Tes review mencakup ACL, office/role, pagination, null profile/draft data, escaping query, canonical customer, attestation, claim race, invalid technical JWT, reconciliation, route default-off/JWT. Seluruh validasi memakai stubs/sqlmock; tidak ada request CCH atau business DB write.
- Parity snapshot dari unique controller `src/routes.php`: BE baru masih parsial. Status/distribusi writes, dokumen, follow-up writes, customer CRUD/identity proof, master CRUD, reports/dashboard, user/role/settings, help/announcement/navigation, free-chat AI, Track & Trace, dan FE/AI integration belum ported. Draft schema tidak punya office column (asumsi satu configured intake office per DB). Reviewer username/attestation/timestamp tersimpan di metadata JSON draft, tetapi immutable audit event terpisah dan retensi PII belum ada; edge rate limit, JWT rotation, dan least-privilege deployment tetap blocker production.

### Handoff Final 2026-10-01

- Superseding validation 2026-10-01: setelah `GET /api/v1/complaints/call-center/:id/status-options` ditambahkan, `go test ./...` dan `go vet ./...` kembali lulus. Endpoint hanya menyajikan opsi aktif 103/109 untuk tiket open asal office Call Center dengan `View cchentri` + `Add cchentrimas`; belum ada status mutation. Generic CCH Add bergantung pada PHP-session master/detail context untuk transaksi distribusi, sehingga status write tetap diblokir sampai ada transaction/idempotency boundary legacy yang teruji.

- Validasi akhir setelah CS review dan parity docs: `go test ./...` dan `go vet ./...` lulus untuk seluruh CCH-BE. Tes seluruhnya memakai test doubles/sqlmock; tidak ada koneksi ke DB CCH, perubahan row bisnis, atau request API CCH nyata. Feature flags complaint/customer/session/submission/review tetap default-off.
- Review attestation disimpan bersama corrected intake payload sebagai metadata internal `_cs_review` (`reviewed_by`, `identity_verified`, `reviewed_at`) saat conditional claim; tidak ada immutable audit event terpisah. Karena draft table tidak punya office column, operasi review saat ini mengasumsikan satu `CCH_INTAKE_OFFICE_CODE` per database bersama.
- `Project-APP-CCH-ke1/` dan file history ini tetap untracked; worktree legacy berisi perubahan lokal lain. Tidak ada stage/commit yang dibuat dan perubahan lain tidak dibersihkan.
- Kelanjutan bisnis paling kritis: trace dan implementasikan transisi status `cchentrimas` + `cchentridet` + sinkronisasi `cchentri` dengan transition matrix per role/office, concurrency, transaksi/partial commit, dan acceptance tests. Setelah itu lanjut dokumen, follow-up writes, CRUD customer/master, reporting/admin, dan AI/FE integration. Jangan klaim parity/production-ready sampai matriks parity ditutup serta retention, edge rate limiting, JWT rotation, dan DB/CCH least privilege disetujui.

- Kelanjutan BE 2026-10-01 - status options Call Center: `GET /api/v1/complaints/call-center/:id/status-options` ditambahkan sebagai read-only endpoint di domain `complaints`, mengikuti legacy origin-office branch: status aktif `103/109`, parent tidak closed, `Kantor_Asal` exact match, reviewer role tunggal 3, `View cchentri`, `Add cchentrimas`, dan office/role konsisten antar ACL. Route ikut complaint-read flag/JWT; inaccessible/closed ticket menjadi 404. SQL/service/handler/composition tests memakai mocks.
- Status mutation sengaja belum diimplementasikan. Generic legacy `/api/add/cchentrimas` memang dispatch ke generated page/hooks, namun `getCurrentDetailTable()` membaca PHP session; transaksi `addRow()` untuk `cchentrideti` hanya dimulai bila master/detail session tersedia. Tanpa konteks session, distribusi bisa terpisah dari histori; `rowInserted()` juga update snapshot `cchentri` setelah insert. Gateway Go saat ini tidak establish konteks session tersebut. Perlu dedicated legacy API action atau kontrak transaksi/idempotency terbukti sebelum status write dibuka. `GET status-options` memberi choices saja, bukan membuktikan transisi berhasil. Tes/regression setelah slice: go test full dan go vet dijalankan setelah patch akhir.

- Backend completion policy 2026-10-01: audit route legacy menemukan 95 controller families unik. Itu tidak berarti membuat 95 controller Go satu-per-satu; parity harus dibuktikan per workflow/domain dengan owner tables/hooks, ACL/office policy, API contract, consistency/idempotency, acceptance tests, operational gates, dan FE handoff contract. FE baru dimulai setelah phase contract stabil. Roadmap phase tetap: complaints/status, follow-ups, documents, customers/masters, completed/reporting, administration/support, lalu AI/external integrations. Legacy PHP hanya sumber analisis pada fase ini; tidak diubah sebagai bagian dari pekerjaan BE baru.

### Kelanjutan BE 2026-10-01 - Call Center Status Transaction

- Supersedes the earlier status-mutation-not-implemented checkpoint above. Added a narrow Go-owned Call Center status writer: open ticket at the exact origin office, role `3`, `View cchentri` + `Add cchentrimas`, expected current status check, and active target `103` or `109`. Other roles/transitions are not implemented; FE remains untouched.
- One MySQL transaction claims a durable idempotency key, locks/revalidates the parent and active status, writes `cchentridet` distributions and `cchentrimas` history, updates `cchentri` status snapshot/request status, and writes duration. History IDs are monotonic per ticket using the locked latest ID. Exact retries replay the stored result; a reused key with changed request conflicts.
- Added `migrations/0001_create_status_transition_idempotency.sql`. Startup requires this table, a valid staff JWT verifier, `CCH_ENABLE_CALL_CENTER_STATUS_WRITE_API=true`, and the operator assertion `CCH_LEGACY_STATUS_WRITES_FENCED=true`; both settings default false. The fence flag does not verify infrastructure configuration. Never run the Go and legacy status writers concurrently.
- HTTP contract: `POST /api/v1/complaints/call-center/{id}/status`, strict JSON allowlist (`expected_status_code`, `status_code`, `description`), required `Idempotency-Key`, no-store response; `201` new write, `200` replay, `409` stale status/key conflict. Route is independent of the complaint-read flag.
- Focused domain/repository/handler/config/composition tests cover auth, strict payloads, exact replay, stale/closed scope, monotonic IDs, rollback injection at write stages, independent gates, and migration startup checks. Tests use sqlmock. No migration was applied, no CCH DB was accessed, and no business write was issued. Live MariaDB integration/concurrency/failure tests and deployment fence verification remain prerequisites; keep route off.
- Documentation updated in `Project-APP-CCH-ke1/CCH-BE/README.md` and `Project-APP-CCH-ke1/CCH-BE/docs/cch-domain-workflow.md`. This history file and the Go project tree are untracked workspace notes/artifacts; nothing staged or committed.

### Kelanjutan BE 2026-10-01 - Call Center Follow-Up Plan

- Added the first follow-up write slice: `POST /api/v1/complaints/call-center/{id}/follow-ups`, independently gated by `CCH_ENABLE_CALL_CENTER_FOLLOWUP_PLAN_API` (default false). It creates a plan only; edit, cancel, and realize are not included.
- Requires current `View cchentri` + `Add cchfu`, exactly Call Center role `3`, consistent DB-revalidated office/report-to/role, open parent visible by origin or exact `Semua_Tujuan`, and a valid `cchfust` code. Client cannot set complaint ID, office, planner, or realization fields; office and actor come from trusted access.
- Repository locks the parent, validates follow-up status, and inserts the plan in one InnoDB transaction. The legacy `(ID_Pengaduan, Kantor, Status)` primary key arbitrates duplicate plans; duplicates map to HTTP 409. No additional schema migration required.
- Domain/service, SQL adapter, HTTP handler, config, and route composition tests added. Tests use sqlmock/stubs only; no live DB query/write or PHP/FE edit. Follow-up write route remains disabled until a separately approved rollout and live integration verification. README and domain workflow docs updated; history and CCH-BE remain untracked, no stage/commit.
- Follow-up realization added separately: `POST /api/v1/complaints/call-center/{id}/follow-ups/{status}/realize`, gated by `CCH_ENABLE_CALL_CENTER_FOLLOWUP_REALIZE_API` (default false). It requires `Edit cchfu` and single role `3`; office comes from revalidated account access. The repository locks the office-owned row and updates realization date/actor/outcome transactionally. Same actor+same outcome retry replays; different actor/outcome conflicts. No migration, live DB write, PHP edit, or FE edit. Plan edit/cancel and other role scopes remain.
- Superseding validation after both follow-up actions and the status writer: focused follow-up/config/cmd-api tests, `go test ./...`, `go vet ./...`, and editor diagnostics passed. All SQL tests use sqlmock; no CCH database, live file storage, migration application, or business row was accessed/changed. All write flags remain default-off.
- Document lifecycle discovery 2026-10-01: legacy globals allow a mixed extension list including Office/ZIP, 2,000,000 bytes per file, unlimited count, and relative `files/` app-root storage; per-ticket folders are created `0777`. Go has no storage root, scanning, retention/deletion, or static-serving fence contract. Upload/download remain unimplemented until secure storage ownership, validation, scanning, retention, and exposure controls are defined; see CCH-BE domain workflow docs.

### Handoff Sesi Berikutnya - 2026-10-01

#### Sudah Dikerjakan

- CCH-BE GoFiber v3 foundation, MySQL adapter/config, health/readiness, JWT verification, DB-revalidated account/role/table permission service, dan authenticated route composition.
- Call Center read slices: complaint open list/detail/history/status options, selected complaint lookups/customer search, serta follow-up related/office/due-today reads. Route opt-in; customer search memakai gate terpisah.
- Guest intake session, submission, dan CS review; ticket write tetap lewat legacy CCH Add gateway, dengan profile matching/review dan reconciliation pada draft.
- Status transaction slice: `POST /api/v1/complaints/call-center/{id}/status`, role Call Center di origin office, `View cchentri` + `Add cchentrimas`, open expected-status check, active target `103/109`, durable idempotency, satu transaksi atas distribution/history/parent snapshot/duration, dan tests failure-injection memakai sqlmock.
- Follow-up plan create: `POST /api/v1/complaints/call-center/{id}/follow-ups`, gated dengan `CCH_ENABLE_CALL_CENTER_FOLLOWUP_PLAN_API`; role 3, `View cchentri` + `Add cchfu`, kantor actor dari ACL, open parent scope, cchfust validation, transactional insert, composite-key duplicate protection.
- Follow-up realization: `POST /api/v1/complaints/call-center/{id}/follow-ups/{status}/realize`, gated dengan `CCH_ENABLE_CALL_CENTER_FOLLOWUP_REALIZE_API`; role 3 + `Edit cchfu`, current office only, transactional row lock/update; same actor and outcome replay, different actor/outcome conflict.
- Document metadata read-only: `GET /api/v1/complaints/call-center/{id}/documents`, independently gated by `CCH_ENABLE_CALL_CENTER_DOCUMENT_METADATA_API` (default false). Requires `View cchentri`, `List cchentridok`, role 3, matching ACL office/report-to, and open-ticket origin/exact-target visibility. Returns type/history/upload office/time/file count only; no filename, folder/path, or bytes. Visible ticket with no docs returns `[]`; invalid/out-of-scope/closed ticket is hidden as 404. No upload/download route was added.
- README, domain workflow, dan history memuat route contracts, parity, status rollout gates, serta document storage discovery. Tidak ada perubahan FE atau legacy PHP.
- Tes terakhir pada workspace aktual setelah document metadata slice (2026-10-01): `go test ./...` lulus; `go vet ./...` lulus. Tests persistence memakai mocks/sqlmock.

#### Belum Dikerjakan

- Belum ada koneksi/query/write Go ke schema bisnis CCH, belum diterapkan migration `migrations/0001_create_status_transition_idempotency.sql`, dan belum ada live MariaDB integration/concurrency/failure test.
- Semua business write flags default-off. Status writer memerlukan operator membuktikan fence jalur legacy `/api/add/cchentrimas`; env `CCH_LEGACY_STATUS_WRITES_FENCED=true` hanya assertion, bukan verifikasi. Rotasi legacy JWT key dan DB least privilege juga belum dibuktikan.
- Follow-up plan edit/cancel, scope role selain Call Center, dan aksi follow-up lain belum ada.
- Dokumen upload/download/preview/delete belum dibuat. Metadata list saja tersedia. Keputusan yang belum ada: private storage owner/root di luar webroot, extension/MIME/size/count limits, malware scanning, retention/deletion, serta penutupan static access. Jangan menyalin `0777` atau global PHP upload allowlist.
- Customer CRUD/identity proof; master CRUD; completed-ticket workflows; reports/dashboards; user/role/settings/admin/support; free-chat AI, Track & Trace, dan integrasi Go-FE-FastAPI belum parity.
- FE sengaja belum dimulai. Seluruh route write/read tetap perlu rollout review; intake PII retention, edge rate limit, JWT rotation, observability, dan backup/recovery belum diselesaikan.

#### Langkah Lanjut

1. Mulai dari kontrak secure document storage dan document ownership; jangan buka binary route sebelum kebijakan storage/security disepakati dan dites.
2. Definisikan secure storage owner/root di luar webroot, static-serving fence, malware scanning, retention/deletion, dan file policy sebelum membuat upload/download workflow. Metadata list sudah tersedia dan tidak mengungkap file/path.
3. Lanjutkan customer/master domains, completed/reporting, administration/support, lalu AI/external integrations berdasarkan legacy trace dan acceptance tests.
4. Sebelum mengaktifkan status write: migration di DB test terisolasi, concurrency/failure tests MariaDB, verifikasi fence legacy, rotasi JWT, dan least-privilege DB account.
5. Jangan mulai FE sebelum workflow kontrak stabil, error DTO/API contract tersedia, route gates dan fixture tests siap, serta parity/security gates disetujui.

#### Kondisi Worktree

- `Project-APP-CCH-ke1/` dan file history ini masih untracked. Tidak ada file di-stage atau commit yang dibuat; tidak ada perubahan legacy PHP/FE atau data DB.

### Kelanjutan BE 2026-10-01 - Document Metadata

- Added `GET /api/v1/complaints/call-center/{id}/documents` as a metadata-only read in `internal/modules/documents`; gated independently by `CCH_ENABLE_CALL_CENTER_DOCUMENT_METADATA_API` (default false).
- Requires `View cchentri`, `List cchentridok`, single Call Center role `3`, matching DB-revalidated office/report-to/role, and an open complaint visible by origin or exact `Semua_Tujuan`. Repository joins document rows to an existing `cchentrimas` history row and uses bounded pagination.
- Response fields are complaint/history ID, document type, upload office, added timestamp, and number of files. It intentionally omits stored file names, `Folder`, download URL, and file bytes. A visible ticket without documents returns `[]`; missing, closed, or out-of-scope ticket maps to 404. Binary upload/download routes are not registered.
- Focused documents service/repository/handler/config/composition tests passed, followed by `go test ./...` and `go vet ./...`. All DB tests use sqlmock. No CCH DB access/write, storage access, PHP change, or FE change.
- Remaining document lifecycle: define private storage root/owner, malware scanner, accepted file policy and caps, retention/deletion, and static-serving fence. Do not enable upload/download until those controls and a MariaDB/storage integration test exist.

### Catatan Hasil Kerja BE - 2026-10-01

Bagian ini mencatat hasil kerja aktual setelah handoff sebelumnya. FE dan legacy PHP tidak diubah. Tidak ada migration yang diterapkan, tidak ada query/write business ke database CCH live, dan tidak ada commit atau staging.

#### Slice yang Sudah Dikerjakan

- **Secure document storage:** private storage root di luar public root, opaque storage key, file validation untuk PDF/JPEG/PNG, batas 2 MB, ClamAV ingestion untuk upload, SHA-256 verification, metadata persistence, dan verified binary download. Upload dan download memiliki feature flag terpisah; download tidak bergantung pada ClamAV. Upload/download/delete/preview/retention belum seluruhnya menjadi lifecycle lengkap.
- **Customer canonical lookup:** `GET /api/v1/complaints/call-center/customers/{id}` ditambahkan di domain `customers`. Requires `Lookup cchentricus`, role Call Center `3`, office dari ACL database, dan mengembalikan `404` yang sama untuk profile tidak ditemukan atau di luar office. Response customer memakai `private, no-store`. Ini bukan identity proof atau phone ownership proof.
- **Follow-up plan lifecycle Call Center:** create, realization, cancellation, dan edit sudah tersedia sebagai route terpisah dan default-off. Create memakai `Add cchfu`; realization/edit memakai `Edit cchfu`; cancellation memakai `Delete cchfu`. Parent dan row follow-up dikunci dalam transaksi; realized plan tidak dapat diedit atau dibatalkan; duplicate/replay/conflict/closed-scope cases dites dengan mocks/sqlmock.
- **Completed complaint read:** `GET /api/v1/complaints/call-center/completed` ditambahkan sebagai read-only slice. Policy terpisah dari open-ticket list: status harus `109`, role Call Center `3`, dan routing memakai `Kantor_Asal`, `Kantor_Tujuan`, atau exact token `Kantor_Tembusan`. Ini mengikuti `cchselesai` sebagai view status `109`, bukan tabel arsip baru.
- **Parity documentation:** README dan `docs/cch-domain-workflow.md` diperbarui untuk route completed, follow-up edit/cancel, document storage, dan current rollout limits. Beberapa parity summary lama masih perlu dirapikan agar seluruh flag baru tercermin konsisten.

#### Route BE yang Sekarang Ada

- Complaint: open list, completed list, open detail, history, status options, lookups, dan narrow status mutation.
- Customer: scoped search dan canonical profile lookup.
- Documents: metadata list, upload, verified download.
- Follow-up: related/office/due-today reads, create, realize, edit, cancel.
- Intake: guest session, guest submission, CS review queue/detail/customer search/create-ticket.

Semua business route tetap default-off dan dipasang melalui composition root sesuai prerequisite masing-masing.

#### Validasi Terakhir

- `go test ./...` lulus pada workspace aktual.
- `go vet ./...` lulus tanpa output error.
- Focused tests untuk complaints, customers, documents, followups, config, handler, repository, dan route composition lulus.
- Persistence tests masih memakai test doubles/sqlmock. Belum ada live MariaDB integration, concurrency, storage deployment, atau request legacy CCH nyata.

#### Yang Masih Belum Lengkap

- **Production/integration gate:** migration `0001` dan `0002` belum diterapkan; belum ada live MariaDB test, lock contention test, commit uncertainty recovery, real ClamAV/storage test, backup/recovery verification, observability, edge rate limiting, retention worker, atau rollback procedure.
- **Status parity:** status writer baru untuk Call Center origin office, role `3`, target `103/109`. Role lain, transition matrix penuh, dan legacy writer fence verification belum selesai. `CCH_LEGACY_STATUS_WRITES_FENCED=true` hanya assertion operator.
- **Complaint parity:** completed detail/history, sent/received reports, dashboard counters, full role-specific list predicates, complaint CRUD, dan semua status workflow belum dipindahkan.
- **Customer/master:** customer create/edit/delete, identity verification, phone ownership proof, serta CRUD master kantor, jenis pengaduan, sumber, layanan, status/handling, dokumen, dan master lain belum ada.
- **Documents:** delete, preview, retention/deletion, production scanner deployment, static-serving fence, dan live storage integration belum ada.
- **Follow-up:** scope role selain Call Center, regional/global assignment, dan workflow follow-up yang lebih luas belum ada.
- **Intake:** idempotency belum atomic dengan legacy CCH API, PII retention belum disetujui, immutable audit event CS belum ada, draft belum menyimpan office terpisah, dan edge rate limit belum ada.
- **Administration/support:** users, roles, settings, regional/CS officer, announcement, help, breadcrumb/navigation, language/timezone belum ported.
- **AI/external:** free-chat orchestration, FastAPI integration, Track & Trace/IPOS, WhatsApp, Kafka, sentiment, categorization, dan AI routing recommendation belum ported.
- **FE handoff:** OpenAPI/equivalent contract lengkap, fixtures, compatibility policy, dan error contract lintas seluruh workflow belum final. FE tetap ditunda.

#### Alur Kerja CCH yang Harus Dipertahankan

```text
Legacy route/controller
	-> PermissionMiddleware / JWT
	-> page atau API action
	-> model hooks
	-> owner table/view
	-> history/distribution/summary side effects
	-> transaction boundary
	-> office/role policy
	-> Go service/repository/handler
	-> acceptance tests
	-> MariaDB integration test
	-> FE contract
```

Untuk setiap slice baru, jangan membuat CRUD generik sebelum route legacy, model hook, owner tables, side effects, office scope, action permission, transaction, idempotency, dan acceptance tests ditentukan. `cchselesai`, dashboard, follow-up, complaint list, dan status history harus tetap memiliki policy terpisah; jangan memakai satu `OfficeScope` generik.

#### Urutan Lanjutan yang Disepakati

1. Selesaikan completed-ticket detail/history dan reporting predicates berdasarkan trace `cchselesai` serta dashboard legacy.
2. Tambahkan live MariaDB integration/concurrency/failure tests sebelum business write flag dibuka.
3. Implementasikan customer/master domain satu workflow per tahap dengan validation dan audit legacy yang ditelusuri.
4. Lanjut administration/support.
5. Setelah contract BE stabil, baru siapkan FE handoff dan integrasi AI/external.

Status history ini: file history dan `Project-APP-CCH-ke1/` tetap workspace artifacts/untracked sesuai catatan sebelumnya. Tidak ada file yang di-stage atau di-commit.

### Kelanjutan BE 2026-10-01 - Completed Ticket Detail

- Completed-ticket read sekarang memiliki list dan detail Call Center: `GET /api/v1/complaints/call-center/completed` dan `GET /api/v1/complaints/call-center/completed/{id}`.
- Detail memakai predicate eksplisit `Status_Akhir = '109'` dan routing `Kantor_Asal`, `Kantor_Tujuan`, atau exact token `Kantor_Tembusan`, mengikuti `cchselesai` dan `Cchselesai::recordsetSelecting()` untuk Call Center. Policy ini sengaja terpisah dari open-ticket detail/list yang memakai origin atau `Semua_Tujuan`.
- Requires `View cchentri`, role tunggal Call Center `3`, dan office dari ACL database. Unknown, closed-ineligible, dan out-of-scope record disembunyikan sebagai not found. Read-only; completed history, reports, dan dashboard counters belum dibuat.
- Added domain repository contract, service method, SQL projection/predicate, HTTP handler route, repository regression test, dan handler test-double support. `go test ./...` dan `go vet ./...` lulus; tidak ada query/write ke CCH live.

### Kelanjutan BE 2026-10-01 - Completed Ticket History

- Completed-ticket read sekarang mencakup `GET /api/v1/complaints/call-center/completed`, `GET /api/v1/complaints/call-center/completed/{id}`, dan `GET /api/v1/complaints/call-center/completed/{id}/history`.
- History memakai parent visibility policy yang sama dengan completed `cchselesai`: status parent `109`, role Call Center `3`, dan office cocok dengan `Kantor_Asal`, `Kantor_Tujuan`, atau exact token `Kantor_Tembusan`. History child tidak diberi office filter terpisah untuk Call Center, sesuai `cchselesaimas` legacy; hasil diurutkan `Tanggal_Update ASC, ID_Histori ASC`.
- Added repository contract/SQL, service policy, handler route, regression predicate test, dan test-double support. `go test ./...` dan `go vet ./...` lulus; tidak ada live DB access atau business write.
- Completed reporting, dashboard counters, dan role predicates selain Call Center tetap belum dibuat.

### Kelanjutan BE 2026-10-01 - Completed Dashboard Count

- Added `GET /api/v1/complaints/call-center/completed/count` as a read-only dashboard count slice behind the existing complaint-read flag. It requires `List cchentri` and role Call Center `3`.
- The count intentionally does not reuse the completed list predicate. It follows legacy `GetJumlahSemuaPengaduanSelesai`: source view `cchselesai`, status-109 semantics, `Kantor_Asal OR exact Semua_Tujuan` membership, and optional `from`/`to` date range over `Tanggal_Pengaduan`.
- Added `CompletedCountQuery`, service/repository/controller contract, SQL predicate regression test, and route test-double support. `go test ./...` and `go vet ./...` lulus; no live DB access or business write.
- This is one dashboard count only. Received/sent/open counters, role-specific dashboard variants, reports/export, and completed aggregations remain unfinished.

### Kelanjutan BE 2026-10-01 - Open Dashboard Count

- Added `GET /api/v1/complaints/call-center/open/count` as a read-only dashboard count behind the complaint-read flag. It requires `List cchentri` and role Call Center `3`, with optional `from`/`to` complaint-date filters.
- The predicate follows legacy `GetJumlahSemuaPengaduanTerbuka`: `Status_Akhir <> '109'` and `(Kantor_Asal = office OR exact token in Semua_Tujuan)`. It is intentionally separate from the open list projection and from completed list/count policies.
- Added open count query/service/controller contract, SQL predicate regression test, handler/service test-double support, and documentation. `go test ./...` and `go vet ./...` lulus; no live DB access or business write.
- Received-open count, sent-open count, role-specific dashboard variants, reports/export, and other aggregates remain unfinished.

### Handoff Sesi Berikutnya - CCH-BE 2026-10-01

#### Status Pekerjaan Terakhir

BE sudah memiliki vertical slices berikut:

- Foundation GoFiber, MySQL/MariaDB adapter, health/readiness, request ID, error envelope, JWT verifier, dan DB-revalidated ACL.
- Call Center complaint reads: open list/detail/history/status-options/lookups.
- Completed reads: completed list/detail/history dengan policy status `109` dan routing `cchselesai`/`cchselesaimas`.
- Dashboard counts: completed count mengikuti `Kantor_Asal OR Semua_Tujuan`; open count mengikuti `Status_Akhir <> '109'` dan `Kantor_Asal OR Semua_Tujuan`; keduanya mendukung filter tanggal opsional.
- Customer scoped search dan canonical profile lookup.
- Follow-up related/office/due-today reads, create, realize, edit, cancel untuk role Call Center.
- Document metadata list, private storage, ClamAV upload ingestion, SHA-256 verification, idempotent metadata write, dan verified download dengan flag upload/download terpisah.
- Guest intake session, submission, CS review, profile matching, reconciliation, dan legacy CCH ticket gateway.
- Narrow Call Center status transaction writer dengan idempotency, parent/history/distribution transaction, default-off gate, dan legacy fence assertion.

#### Validasi Terakhir

- `go test ./...` lulus.
- `go vet ./...` lulus tanpa output error.
- Tests persistence masih memakai sqlmock/test doubles.
- Tidak ada live MariaDB query/write, migration application, request CCH nyata, atau storage deployment test.
- FE dan legacy PHP tidak disentuh.

#### Pekerjaan Berikutnya

1. Tambahkan received-open count dan sent-open count dengan predicate legacy masing-masing; jangan memakai predicate dashboard generik.
2. Tambahkan role-specific dashboard variants dan report/export setelah predicate legacy setiap report ditrace.
3. Lengkapi customer/master workflow satu per satu: validation, ACL, office scope, audit, transaction, dan acceptance tests.
4. Siapkan live MariaDB integration/concurrency/failure tests sebelum business write flag diaktifkan.
5. Selesaikan document delete/preview/retention/backup/static-serving fence.
6. Lanjut administration/support, kemudian AI/external integration setelah kontrak BE stabil.
7. FE tetap ditunda sampai API/error contract, fixtures, feature gates, dan parity/security gates disetujui.

#### Aturan Melanjutkan

- Mulai dari legacy route/controller/model hook dan owner table/view.
- Pisahkan policy complaint list, dashboard, completed, status history, dan follow-up; jangan membuat generic `OfficeScope`.
- Jangan membuat CRUD generik sebelum validasi legacy, side effects, action permission, office/role scope, transaction, idempotency, dan acceptance tests jelas.
- Semua route bisnis tetap default-off.
- Jangan menerapkan migration, memakai root, membaca data pelanggan, atau melakukan business write live tanpa tahap integration test dan rollout approval.

#### Kondisi Worktree

History ini dan `Project-APP-CCH-ke1/` tetap workspace artifacts/untracked. Tidak ada file yang di-stage atau di-commit. Legacy PHP, FE, dan data database tidak diubah dalam pekerjaan ini.

### Kelanjutan CCH-BE 2026-10-02 - Dashboard dan Export

- Added Call Center received-open and sent-open dashboard counts. Legacy functions verified from `GetJumlahPengaduanTerimaTerbuka` and `GetJumlahPengaduanKirimTerbuka`: both require `Status_Akhir <> '109'`; received scopes only by exact `Semua_Tujuan` office token, sent only by `Kantor_Asal`. Optional date range filters `Tanggal_Pengaduan`. Counts do not reuse the combined open predicate.
- Added role-aware `/api/v1/complaints/dashboard/{open,received-open,sent-open,completed}/count` routes behind the existing default-off complaint read registrar. Only one office role is accepted (`1,2,3,9,10,19,20,29`), all require `List cchentri`, and office comes from DB-revalidated access. Multi-role, unknown roles, and System Administrator remain denied; no global scope is exposed.
- Added bounded `GET /api/v1/complaints/call-center/open/export` CSV slice (maximum 100 rows), requiring both `List` and `Export cchentri`, Call Center role `3`, trusted office/open predicate, fixed projection without customer contact fields, no-store attachment headers, and spreadsheet formula neutralization.
- Added `mysql_mariadb_integration_test.go`: requires `CCH_TEST_MARIADB_DSN` targeting an empty disposable database named with `cch_be_test_` prefix. It creates fixture InnoDB tables, validates dashboard counts/exact CSV membership, and exercises concurrent expected-status conflicts and exact-key replay for the status writer. The variable was not configured during this session; the integration test compiled and skipped. No MariaDB connection, migration, or live write occurred.
- Report audit found `cch_rekap_terima` combines regional/product/date/destination-only filters, duration buckets, detail links, and Excel output; only the narrow open-ticket CSV is implemented. Broader report contracts/exports remain separate work.
- Customer search/profile lookup and intake master lookups already exist. Customer mutations remain deferred: legacy profile create checks name duplicates globally without a database unique constraint/lock; profile matching is not identity/phone ownership proof. Identity verification, race-safe duplicate arbitration, field policy, office ownership, and audit contract are required before writes. No new customer/master CRUD was added.
- Updated BE README/domain workflow documentation. `go test ./...`, `go vet ./...`, focused complaint tests, and editor diagnostics passed. PHP, FE, live CCH schema, and business rows were not changed. History and `Project-APP-CCH-ke1/` remain untracked workspace artifacts; nothing staged or committed.

### Kelanjutan CCH-BE 2026-10-02 - Received/Sent Detail Reports

- Added paginated `GET /api/v1/complaints/dashboard/received-open` and `/sent-open` detail lists using the existing minimal complaint-summary response. Both support optional `from`/`to`, default/max pagination, require `List cchentri`, one supported DB-revalidated office role, and derive office from ACL rather than caller input. Routes share the default-off complaint read registrar and return `private, no-store`.
- Repository predicates are separate and match the corresponding legacy dashboard functions: both require `Status_Akhir <> '109'`; received checks exact token membership in `Semua_Tujuan` only; sent checks `Kantor_Asal` only. Customer-contact/detail columns are not selected.
- Acceptance tests cover exact WHERE scopes/date args, trusted office, role and pagination flow, response metadata, and default-off route behavior. An isolated MariaDB integration fixture now also asserts the received/sent report result IDs and excludes partial-office matches.
- Validation: focused tests (110 passed), `go test ./...`, `go vet ./...`, and editor diagnostics passed. `CCH_TEST_MARIADB_DSN` remains unset, so integration harness compiled/skipped and no live MariaDB was accessed.
- This does not implement regional/product/duration report aggregates or other export formats. Those remain separate slices; no PHP, FE, live schema, migration, or business rows were changed. No files staged or committed.

### Kelanjutan CCH-BE 2026-10-02 - Received-Open Detail Report

- Added `GET /api/v1/complaints/reports/received-open`, a narrow detail slice traced to legacy `CchRincianTerimaTerbuka`. It requires `from` and `to`, accepts optional `product_code` (`999` means all), and uses the authenticated account office only; callers cannot select another office/regional scope.
- Query requires `cchentridet.Status_Update = '101'` and exact `Kantor_Tujuan_Update` match, non-109 parent and latest history status; latest history is deterministic by `Tanggal_Update DESC, ID_Histori DESC`. It filters optional product and `cchentri.Tanggal_Tambah`, returns bounded rows with latest status/office/time and elapsed seconds, and omits customer contact fields.
- Service requires `List` on `cchentri`, `cchentridet`, and `cchentrimas`, one supported office role, and matching current office/report-to/role across ACL checks. Handler is behind the existing complaint-read feature gate and returns private no-store JSON.
- Added repository SQL/scan tests, three-table ACL and denial tests, handler projection/date tests, default-off composition coverage, and MariaDB fixture coverage. `go test ./...`, `go vet ./...`, focused complaint tests, and editor diagnostics passed. `CCH_TEST_MARIADB_DSN` remains unset, so the integration test was only compiled/skipped; no live DB or business writes occurred.
- Regional office selection, product/duration aggregate breakdowns, legacy localized duration formatting, and report exports are not part of this slice. No PHP/FE changes, migration, staging, or commit.

### Kelanjutan CCH-BE 2026-10-02 - Received-Open Duration Buckets

- Added `GET /api/v1/complaints/reports/received-open/duration-buckets` for the verified current office. Buckets are calendar age `0`, `1`, `2`, `3`, and `4+` days from `Tanggal_Tambah` to MariaDB `CURRENT_DATE`; response includes each count, two-decimal percentage, and total. Empty totals return zero percentages.
- Query includes distinct tickets with an initial `cchentridet` distribution (`Status_Update = '101'`) to the exact current office, exact token membership in `Semua_Tujuan`, `cchentri.Status_Akhir <> '109'`, required date range, and optional product (`999` means all). It uses parameterized predicates and does not read customer data.
- Service requires `List cchentri` and `List cchentridet`, one supported office role, and matching office/report-to/role across ACL reads. Route is behind the existing default-off complaint read gate; no office or regional scope can be supplied by the caller.
- Acceptance tests cover five SQL buckets/parameters, `999` normalization, invalid office/date rejection, ACL denials, response percentages/no-store, and default-off routing. MariaDB integration fixture seeds dates relative to `CURRENT_DATE` for all five boundaries, but `CCH_TEST_MARIADB_DSN` remains unset; live MariaDB tests were not run. Full `go test ./...`, `go vet ./...`, and editor diagnostics passed.
- Regional office grouping/selection, links to detailed records, alternate legacy day-age semantics, and Excel export remain unported. No PHP/FE/database business changes, migration, staging, or commit.

### Kelanjutan CCH-BE 2026-10-02 - Complaint Type Master CRUD

- Added a default-off `CCH_ENABLE_COMPLAINT_TYPE_MASTER_API` CRUD vertical slice for legacy `cch_jenis_pengaduan`: `GET/POST /api/v1/masters/complaint-types`, `GET/PUT/DELETE /api/v1/masters/complaint-types/:code`. It is isolated in `internal/modules/masters/complainttypes` with domain service, repository interface/MySQL adapter, and HTTP handler.
- Authorization is action-specific against the CCH ACL table: List, View, Add, Edit, Delete. No role-name bypass is present; legacy default ACL entries are deny. All routes require JWT and DB composition when enabled.
- API validates signed TINYINT codes `-128..127`, nullable description up to 50 runes, strict JSON body fields, and active boolean mapped to DB `Y/N`. The key is immutable on update. DELETE intentionally soft-deactivates and is idempotent to preserve historical ticket references; create/update set `Update_Terakhir` server-side. Duplicate code maps to 409.
- Added repository SQL/mock tests, service action/validation tests, handler CRUD/strict-body/auth tests, config flag and default-off/JWT composition tests. Full `go test ./...`, `go vet ./...`, and editor diagnostics passed. No live DB, master-row writes, PHP/FE edits, stage, or commit.
- This establishes minimum CRUD for complaint type only. Office/source/service/status masters, customer CRUD/identity proof, role-specific permission provisioning, and MariaDB integration remain. Keep gate disabled until intended management ACLs are provisioned and live acceptance tests are run.

### Kelanjutan CCH-BE 2026-10-02 - Complaint Source Master CRUD

- Added separate source domain `internal/modules/masters/sources` with `GET/POST /api/v1/masters/complaint-sources`, `GET/PUT/DELETE /api/v1/masters/complaint-sources/:id`. `CCH_ENABLE_COMPLAINT_SOURCE_MASTER_API` is default-off and independent from complaint-type master CRUD.
- Schema contract follows legacy `cch_sumber_pengaduan`: auto-increment integer ID, required `Sumber` up to 100 chars, `Apakah_Aktif` `Y/N`, and server-managed `Tanggal_Tambah`/`Tanggal_Ubah`. Delete is an idempotent soft deactivation to preserve ticket references. No unique source-name constraint was invented because legacy has none.
- Every operation checks its corresponding table action (`List`, `View`, `Add`, `Edit`, `Delete`) via current CCH ACL. No role-name bypass exists; default ACL entries remain deny. Routes require DB and staff JWT composition when enabled.
- Added repository, service, handler, config and route-composition tests. Focused tests, `go test ./...`, `go vet ./...`, and editor diagnostics passed. Tests use sqlmock/stubs only; no live master row, PHP/FE, or DB schema was modified. No files staged or committed.
- Complaint-type, complaint-source, and shipping-service CRUD are implemented as separate master slices. Office, status/handling, document-type/other master CRUD, customer CRUD/identity proof, ACL provisioning, and live MariaDB acceptance remain outstanding. Keep all master flags off until intended ACLs are provisioned and isolated integration is verified.

### Kelanjutan CCH-BE 2026-10-02 - Shipping Service Master CRUD

- Added `internal/modules/masters/shippingservices` with `GET/POST /api/v1/masters/shipping-services`, `GET/PUT/DELETE /api/v1/masters/shipping-services/:code`, behind independent default-off flag `CCH_ENABLE_SHIPPING_SERVICE_MASTER_API`.
- Follows legacy `cch_jenis_layanan`: `Kode_Layanan CHAR(3)`, `Deskripsi VARCHAR(25)`, `URL_Lacak VARCHAR(255)`, `Waktu_Akhir`, and `Status_Aktif Y/N`. Code is immutable on update; create/edit set `Waktu_Akhir`; delete soft-deactivates. URL is stored only and never fetched by BE.
- Each operation checks the corresponding table ACL action; enabled composition requires DB and staff JWT. Legacy and existing BE intake lookup include inactive shipping services, so lookup behavior remains unchanged in this slice.
- Added repository, service, handler, config-flag and default-off/JWT composition tests. Focused tests, `go test ./...`, and `go vet ./...` passed. Tests use mocks only; no live DB rows, PHP/FE changes, migration, staging, or commit.
- Added repository, service, handler, config-flag and default-off/JWT composition tests. Focused tests, `go test ./...`, and `go vet ./...` passed. Tests use mocks only; no live DB rows, PHP/FE changes, migration, staging, or commit.

### Kelanjutan CCH-BE 2026-10-02 - Document Type Master CRUD

- Added `internal/modules/masters/documenttypes` with `GET/POST /api/v1/masters/document-types`, `GET/PUT/DELETE /api/v1/masters/document-types/{code}`, independently gated by default-off `CCH_ENABLE_DOCUMENT_TYPE_MASTER_API`.
- Trace: legacy routes in `src/routes.php` dispatch through `CchJenisDokumenController` and `PermissionMiddleware`; `cch_jenis_dokumen` has a `CHAR(3)` key, required 100-character description, `Status_Aktif` Y/N, and add/update timestamp+actor fields. Row hooks stamp audit values; no other business side effects were found. The `cchentridok` primary key stores `Jenis_Dokumen` without a foreign key, so deactivation is soft (`Status_Aktif='N'`) to preserve historical document references. Description edits change the current lookup label for old records, consistent with the legacy join behavior.
- Each endpoint requires the matching table ACL action (`List`, `View`, `Add`, `Edit`, `Delete`); actor and timestamps are server-owned. Tests cover ACL separation, field constraints, strict HTTP payload/auth, route default-off/JWT/DB gates, SQL audit writes, and idempotent soft-deactivation. Tests use sqlmock/stubs only; no live DB access or changes, no PHP/FE edits, no staging/commit.

### Kelanjutan CCH-BE 2026-10-02 - Handling Status Master CRUD

- Added `internal/modules/masters/handlingstatuses` with `GET/POST /api/v1/masters/handling-statuses`, `GET/PUT/DELETE /api/v1/masters/handling-statuses/{code}`, independently gated by default-off `CCH_ENABLE_HANDLING_STATUS_MASTER_API`.
- Legacy routes dispatch through `CchJenisPenangananController` and `PermissionMiddleware`; `cch_jenis_penanganan` has `ID_Status VARCHAR(3)`, required `Deskripsi_Status VARCHAR(50)`, and nullable `Aktif Y/N`. Hooks add no workflow side effects. Status codes appear in current ticket and history records, so Go DELETE is soft-deactivation and code stays immutable; label edits continue to affect historical joins as they do in legacy.
- Each operation checks its matching action ACL. Acceptance tests cover policy, SQL soft-deactivation, strict HTTP payloads, and default-off composition; execution was deferred per user request. No live DB, PHP/FE changes, stage, or commit.

### Kelanjutan CCH-BE 2026-10-02 - Domain Controller Layout

- Standardized all 12 domain HTTP packages from `handler/` to `controller/`, and moved the health HTTP adapter into `internal/modules/health/controller`. Controller types and specialized constructors now use `Controller` naming; `ReadinessChecker` remains owned by the health domain and is injected into its controller.
- Updated `cmd/api` composition, the intake-session rate-limiter import, and current README/domain workflow package references. No route, request/response contract, ACL, SQL, or business policy was intentionally changed.
- `go build ./...` and `go vet ./...` passed; behavior tests were not run per user request. Editor diagnostics still report cognitive-complexity warnings in existing long functions. No database access/write, staging, or commit.
- `go build ./...` and `go vet ./...` passed; behavior tests were not run per user request. Editor diagnostics still report cognitive-complexity warnings in existing long functions. No database access/write, staging, or commit.

### Kelanjutan CCH-BE 2026-10-02 - Office Master CRUD

- Added `internal/modules/masters/offices` with CRUD plus `GET /api/v1/masters/offices/options`, gated by default-off `CCH_ENABLE_OFFICE_MASTER_API`.
- Trace: legacy `CchKantorController` routes use `PermissionMiddleware`; office row hooks have no write side effects. `cch_kantor` has eight required fields, no audit columns, and no FK. The `Jenis` lookup uses distinct existing values; the Regional selector accepts active `KP`/`Divre` offices. BE validates both references transactionally, restricts Regional codes to the destination column's five-character capacity, keeps office codes immutable, and soft-deactivates to preserve account/ticket references.
- Per-action ACL applies to CRUD; the options endpoint requires `Lookup`. Acceptance tests were added but not run per user request. No live DB/migration, PHP/FE changes, staging, or commit.

### Kelanjutan CCH-BE 2026-10-02 - Office Master CRUD

- Added `internal/modules/masters/offices` with office CRUD and `GET /api/v1/masters/offices/options`, gated by default-off `CCH_ENABLE_OFFICE_MASTER_API` and composed under the `controller/` domain layout.
- Trace: legacy `CchKantorController` routes pass through `PermissionMiddleware`; `cch_kantor` has eight non-null fields, no audit columns, and no FK. Legacy lookup choices expose distinct `Jenis` values and active Regional offices with `Jenis` KP/Divre. Hooks add no other write effects. BE validates those references, bounds `latin1` field values, keeps the code immutable, uses independent List/View/Lookup/Add/Edit/Delete ACL, and soft-deactivates instead of deleting codes referenced by user/ticket data.
- Acceptance tests cover per-action ACL, options and reference rules, soft deletion, strict HTTP input, and default-off DB/JWT gates. Tests were not run per the current schedule. No live DB/migration, PHP/FE change, stage, or commit.

### Kelanjutan CCH-BE 2026-10-02 - Follow-Up Status Master CRUD

- Added `internal/modules/masters/followupstatuses` with `GET/POST /api/v1/masters/follow-up-statuses`, `GET/PUT/DELETE /api/v1/masters/follow-up-statuses/{code}`, and lookup options under default-off `CCH_ENABLE_FOLLOWUP_STATUS_MASTER_API`.
- Legacy `CchfustController` is protected by `PermissionMiddleware`; `cchfust` contains only `Kode_FollowUp CHAR(3)` and nullable `Deskripsi_FollowUp VARCHAR(20)`, with no active flag or row-hook side effects. `cchfu.Status` references the code in its composite key. Code is immutable; delete locks the master and returns conflict while a `cchfu` reference exists, deleting only unused codes.
- CRUD uses per-action ACL, and the options route requires `Lookup`. Acceptance tests cover ACL, null/length/charset constraints, references, HTTP allowlists, and route default-off/DB/JWT gates. Tests were not run per current schedule; no live DB/migration, PHP/FE edit, stage, or commit.

### Kelanjutan CCH-BE 2026-10-02 - Customer Profile CRUD

- Added office-scoped customer profile list/detail/create/update/delete under `internal/modules/customers`, with separate default-off `CCH_ENABLE_CUSTOMER_PROFILE_API` and controller routes `GET/POST /api/v1/customers`, `GET/PUT/DELETE /api/v1/customers/{id}`.
- CRUD requires per-action ACL on `cchentricus`; current office is revalidated from account access and never accepted from request. Legacy uppercase name/address behavior and server audit/office assignment are preserved. List returns total/limit/offset. Profile delete is physical only when no ticket or chatbot intake draft references the ID; otherwise returns conflict. Existing ticket snapshots are not modified.
- Create checks global exact-name duplicate using the existing case-insensitive collation. `cchentricus` has no unique name constraint, so concurrent legacy PHP and Go creates are not race-proof; this is explicitly documented rather than claimed solved. Search matching is not identity or phone ownership proof.
- Acceptance tests cover ACL, office spoofing, validation, pagination, duplicates/reference guards, HTTP allowlist/auth, and route default-off/DB/JWT gates. Tests not run; no live DB or migration, PHP/FE edit, stage, or commit.

### Kelanjutan CCH-BE 2026-10-02 - Help Category Master CRUD

- Added `internal/modules/masters/helpcategories` for the legacy `cch_help_categories` workflow: list/detail/create/update/delete plus language options from `cch_languages`. Routes are gated by independent default-off `CCH_ENABLE_HELP_CATEGORY_MASTER_API` and require staff JWT/database when enabled.
- Trace: legacy routes use `CchHelpCategoriesController` and `PermissionMiddleware`; schema fields are `Category_ID` auto-increment, `Language CHAR(2)`, and `Category_Description VARCHAR(100)`. Insert/update/delete hooks add no side effects. `cch_help.Category` references the current category label without a database FK; Go verifies language references and rejects delete with `409` when help entries use that category. Concurrent legacy writes remain an operational coordination risk because no shared lock/FK exists.
- Acceptance tests cover per-action ACL including `Lookup`, validation, language existence, delete reference protection, config flag parsing, disabled routes, and enabled route JWT/database requirements. Focused tests passed (20 tests). `go vet ./...` passed. `go test ./...` was run but has one unrelated failure in `internal/modules/customers/repository/mysql_profiles_test.go`: expected query order differs from the repository's page/count query order; all help-category packages passed.
- `CCH_TEST_MARIADB_DSN` was not configured. No DB was connected, no migration/business data was changed, and no PHP/FE files were edited. All category repository tests use sqlmock. `Project-APP-CCH-ke1/` and this history file remain untracked workspace artifacts; nothing was staged or committed.

### Kelanjutan CCH-BE 2026-10-02 - Help Article CRUD

- Added `internal/modules/help` with bounded summary list/filter, full detail, lookup options, create, update, and delete routes for legacy `cch_help`, independently gated by default-off `CCH_ENABLE_HELP_ARTICLE_API`. All CRUD operations enforce their matching `List`, `View`, `Add`, `Edit`, or `Delete` ACL; options require `Lookup`.
- Legacy trace: `/cchhelplist`, add/view/edit/delete and `/cchhelppreview` pass through `PermissionMiddleware`; table fields include language, topic, long description, category, order, display page, updater, and update time. Model selection/row hooks add no other scope or side effects. Go validates the `cch_languages` and `cch_help_categories` references under row locks; actor comes from verified identity and update time from MariaDB. List omits LONGTEXT. The API preserves description text/markup and does not render or sanitize it. Physical article delete follows legacy behavior; category deletion is serialized against Go article writes but PHP does not share the lock protocol.
- Acceptance coverage verifies action ACLs, input limits/required fields, strict JSON allowlists, no-store/auth, pagination/filter SQL, full-description retrieval, audit ownership, reference validation, transactional writes, and default-off/DB/JWT route gates. Focused tests passed (30). `go vet ./...` passed. Full `go test ./...` still fails only at the existing `customers/repository/mysql_profiles_test.go` count-vs-page-query expectation; every Help Article package passed.
- Legacy `/cchhelppreview` rendering remains unported. `CCH_TEST_MARIADB_DSN` remains unavailable; tests use sqlmock, with no DB connection, migration, PHP/FE change, stage, or commit. The FE remains deferred by user direction; this does not mark overall BE parity complete.

### Kelanjutan CCH-BE 2026-10-06 - Regional Office Admin and CS Officer Status

- Added `internal/modules/regionals` for legacy `cch_regional`: default-off `CCH_ENABLE_REGIONAL_ADMIN_API` gates `GET /api/v1/admin/regional-offices`, `GET /{code}`, `GET /options`, and `PUT /{code}`. Legacy route exposes list/view/edit only; BE intentionally has no add/delete. The owner is the updatable single-table view `cch_regional` over `cch_kantor` restricted to `Jenis IN ('Divre','KP')`, ordered by abbreviation. List/View/Edit ACL is checked against `cch_regional`; lookup options require `Lookup cch_kantor`. Update keeps the office code immutable (safety difference from the legacy edit form), locks the source row while it is still KP/Divre, locks the referenced parent office, and writes editable name/abbreviation/type/regional/active fields to the underlying `cch_kantor` row in one transaction. Field bounds and latin1 representability follow the base table. This API overlaps `masters/offices` and the legacy PHP writer on the same rows: row locks serialize concurrent DB updates, but stale client writes remain last-writer-wins, so keep the route off until ownership/rollout coordination is approved.
- Added `internal/modules/csofficers` for the legacy read-only `cch_cs_officer` view: default-off `CCH_ENABLE_CS_OFFICER_STATUS_API` gates `GET /api/v1/admin/cs-officers` and `GET /{username}`. The view selects `cch_users.Display_In_CS_List='Y'` and exposes report-to, office, username, name, `Current_Status` Login/Logout, email, phone, and login/logout timestamps. Legacy `recordsetSelecting()` adds no extra scope and controller has no write route; BE preserves that view predicate and requires per-action `List`/`View cch_cs_officer` without inventing office scope. List supports exact regional/office/status filters, escaped username/name prefix search, bounded pagination (25 default/100 max), and private/no-store headers because email/phone are PII. Status is CS login/logout state, not complaint status.
- Acceptance tests cover per-action ACL, invalid filters/inputs, pagination, exact option forwarding, SQL view predicates/columns, nullable contact/time values, SQL wildcard escaping, row locks and reference failures for Regional, no-store/auth/error mapping, and independent default-off/database/JWT route gates. Focused tests passed; `gofmt`, `go build ./...`, `go test ./...`, and `go vet ./...` all passed. SQL tests use sqlmock; no live CCH query/write or migration occurred. `CCH_TEST_MARIADB_DSN` remains unavailable. No PHP/FE edits, staging, or commit.

### Kelanjutan CCH-BE 2026-10-06 - User-Level Catalog Read

- Added `internal/modules/userlevels` as a read-only catalog for `cch_userlevels`, gated by independent default-off `CCH_ENABLE_USER_LEVEL_CATALOG_API`. Routes are `GET /api/v1/admin/user-levels` and `GET /api/v1/admin/user-levels/{id}`; list/detail require `List`/`View` action ACL on `cch_userlevels`, with bounded pagination (default 25, max 100), signed legacy int IDs, 404 for missing detail, and private/no-store responses.
- Legacy `/cchuserlevelslist` and `/cchuserlevelsview` are protected by `PermissionMiddleware`; owner schema has `User_Level_ID INT` and required `User_Level_Name VARCHAR(255)`, with no row-hook side effects. SQL projection contains only ID/name. `cch_userlevelpermissions` is not read or exposed. User/role/permission mutations remain out of scope because user assignments and ACL rows reference level IDs and need a separately approved write/delete/race policy.
- Acceptance tests cover service ACL/validation, sqlmock projection and pagination, HTTP auth/no-store/input/error mapping, and independent default-off/DB/JWT route composition. Focused package/config/route tests passed after wiring. Full `go build ./...`, `go test ./...`, and `go vet ./...` are the remaining verification gates. No live DB access, PHP/FE edits, staging, or commit.

### Kelanjutan CCH-BE 2026-10-06 - Breadcrumb Links Administration

- Added `internal/modules/breadcrumbs` for the legacy `cch_breadcrumblinks` nested-set tree, gated by independent default-off `CCH_ENABLE_BREADCRUMB_LINKS_API`. Routes are `GET /api/v1/admin/breadcrumbs`, `GET /{title}`, `GET /{title}/ancestors`, `POST /api/v1/admin/breadcrumbs`, `PUT /{title}/move`, and `DELETE /{title}`. Each operation requires the matching `List`/`View`/`Add`/`Edit`/`Delete` action on `cch_breadcrumblinks`; responses are private/no-store.
- Legacy routes dispatch through `CchBreadcrumblinksController` and `PermissionMiddleware`; the tree is managed by custom pages `breadcrumblinksaddsp/movesp/deletesp/checksp` with empty stored-procedure shells in `cch.sql`. Go reproduces the page guards (duplicate title, parent/root existence) and the check join `B.Lft BETWEEN C.Lft AND C.Rgt`, and defines nested-set add/move/delete inside one DB transaction because the procedure bodies are not present.
- The service, repository (sqlmock), controller (HTTP/auth/error mapping), config flag, and route composition tests pass once with cache disabled. Remaining: full `gofmt`, `go build ./...`, `go test ./...`, `go vet ./...`, then docs/memory checkpoint. No live DB access, PHP/FE edits, staging, or commit.

### Kelanjutan CCH-BE 2026-10-06 - Sent-Open Detail Report

- Added read-only `GET /api/v1/complaints/reports/sent-open` in the `complaints` domain behind the existing default-off `CCH_ENABLE_CALL_CENTER_COMPLAINT_READ_API`; no new flag was introduced. It mirrors legacy `CchRincianKirimTerbuka` (`views/CchRincianKirimTerbuka.php`), the sent counterpart of the already-ported received-open detail report.
- Legacy source query: `SELECT DISTINCT(u.ID_Pengaduan), e.Kantor_Asal, e.Nomor_Kiriman, e.Jenis_Kiriman, e.Tanggal_Tambah, e.Kantor_Tujuan FROM cchentridet u, cchentri e WHERE e.Kantor_Asal = '<office>' AND u.Status_Update = '101' AND e.Tanggal_Tambah BETWEEN <from> AND <to> [AND e.Jenis_Kiriman = '<product>'] AND u.ID_Pengaduan = e.ID_Pengaduan`; the page then resolves the latest `cchentrimas` (`ORDER BY Tanggal_Update DESC LIMIT 1`) and shows only rows whose latest history status is not `109`. Unlike the received report, the sent query does not filter the parent `Status_Akhir`; open-ness is decided from the latest history status.
- Go slice reproduces that contract: origin-only `cchentri.Kantor_Asal = ?` scope, initial `cchentridet.Status_Update = '101'` distribution, deterministic latest-history subquery (`Tanggal_Update DESC, ID_Histori DESC`), latest history `<> '109'`, required date range, optional `product_code` with `999` sentinel, bounded pagination, and a projection without customer contact fields. Responses are `private, no-store`.
- Service requires `List` on `cchentri`, `cchentridet`, and `cchentrimas` with consistent DB-revalidated office/role/report-to; office comes from account access, never from the caller, and only one supported office role is accepted.
- Acceptance tests: repository sqlmock (exact sent SQL shape/args, product filter, invalid date/office rejection), service (three-table ACL, office mismatch, missing history permission), controller (projection and no-store, missing date bounds), `cmd/api` route composition (default-off 404 and JWT protection when enabled). Isolated MariaDB integration fixture now also asserts the sent-open report result and excludes partial-office matches; `CCH_TEST_MARIADB_DSN` remains unset so that test compiled and skipped.
- Fix: the isolated integration fixture misspelled `cchentridet` as `cchentridiet`, so the received-open report assertion had no distribution rows; the typo was corrected and `TKT-SENT` distribution/history fixtures were added. No live DB, PHP/FE edit, migration, staging, or commit. `gofmt`, `go build ./...`, `go test ./... -count=1`, and `go vet ./...` all pass. Regional/office selection and Excel export from the legacy page remain unported.

### Kelanjutan CCH-BE 2026-10-06 - Sent-Open Duration Buckets

- Added read-only `GET /api/v1/complaints/reports/sent-open/duration-buckets` in the `complaints` domain behind the existing default-off `CCH_ENABLE_CALL_CENTER_COMPLAINT_READ_API`; no new flag. It mirrors the sent-open-report policy and the legacy sent duration pages in spirit: `cchentri.Kantor_Asal` equality, initial `cchentridet.Status_Update = '101'` distribution, deterministic latest `cchentrimas` row, latest history status not `109` (parent `Status_Akhir` not filtered), `from`/`to`, and optional `product_code` (`999` = all).
- Response shape is identical to `received-open/duration-buckets`: `day_zero`..`day_four_plus` counts, two-decimal percentages, and total; calendar age is `DATEDIFF(CURRENT_DATE, DATE(c.Tanggal_Tambah))`. Because the sent open filter reads the latest history row, the endpoint requires `List` on `cchentri`, `cchentridet`, and `cchentrimas` with consistent DB-revalidated office/role/report-to (received buckets require only parent + distribution).
- Implementation: `SentOpenDurationQuery.Normalize()` (`999` sentinel), `SentOpenDurationBuckets` alias of `ReceivedOpenDurationBuckets`, `sentOpenDurationBucketsSQL` with the same five-bucket SUM/CASE projection, `MySQLComplaintRepository.CountSentOpenDurationBuckets`, `Service.CountSentOpenDurationBuckets`, and `Controller.CountSentOpenDurationBuckets` reusing `receivedOpenDurationResponse`.
- Acceptance tests: service (three-table ACL + office mismatch + missing history permission), repository sqlmock (exact SQL shape/args for all-products and single-product, invalid bounds/office before query), controller (counts/percentages/total, no-store, missing dates rejected before service call). Full `gofmt`, `go build ./...`, `go test ./... -count=1`, and `go vet ./...` pass. No live DB, PHP/FE edit, migration, staging, or commit; README and `docs/cch-domain-workflow.md` updated. Legacy sent duration pages still offer per-day filtering (`durasihari`) and regional selector/export that the Go five-bucket slice intentionally does not reproduce.

### Kelanjutan CCH-BE 2026-10-06 - Sent Summary (Rekap Kirim)

- Added read-only `GET /api/v1/complaints/reports/sent-summary` in the `complaints` domain behind the existing default-off `CCH_ENABLE_CALL_CENTER_COMPLAINT_READ_API`; no new flag. It implements the legacy `CchRekapKirim*` family (`CchRekapKirim` plus the `JenisMasalah`/`JenisKiriman`/`Durasi*` variants share the same grouped-by-origin semantics).
- Legacy base query groups `cchentri` by `Kantor_Asal` and per row computes: total (`COUNT`), completed (`Status_Akhir = '109'`), open (`total - completed`, with the MPC-Jakarta clamp where completed > total), unresponded (`Status_Akhir = '101'`), and responded (`total - unresponded`), each with a percentage. The Go slice folds the per-row breakdown into one grouped `sentSummarySQL` with conditional `SUM(e.Status_Akhir = '109')`/`'101'`, joins `cch_kantor` for the office name, and supports optional `product_code` + `regional` (`0`/empty = all; otherwise `k.Regional = ? OR k.Kode_Kantor = ?` matching the legacy `cch_regional` join). Bounded pagination, `ORDER BY Kantor_Asal ASC`, no contact PII, `private, no-store`.
- ACL: `List` on `cchentri` and `cch_kantor` with consistent DB-revalidated office/role/report-to (`isOfficeDashboardRole`), office always taken from account access. Added `SentSummaryQuery.Normalize()` (`999`/`0` sentinels), `SentSummaryRow`, `ListSentSummary` at repository/service/controller layers, and `sentSummaryItem`/`sentSummaryResponse` reusing `reportPercentage` for two-decimal percentages.
- Acceptance tests: service (two-table ACL + office summary pass, missing office list / office mismatch denied), repository sqlmock (exact SQL + args for all-regions/all-products and single product+region, invalid date/region-list/multi-office rejection), controller (JSON projection with percentages, regional/product forwarding, no-store, missing dates rejected). Full `gofmt`, `go build ./...`, `go test ./... -count=1`, and `go vet ./...` pass. No live DB, PHP/FE edit, migration, staging, or commit; README and `docs/cch-domain-workflow.md` updated. Regional detail links and the Excel-HTML export (legacy `?export=excel` prints an HTML table) remain unported; the sent duration variants' per-day (`durasihari`) filtering and regional/product detail drill-downs also remain out of scope for this slice.

### Kelanjutan CCH-BE 2026-10-06 - Received Summary (Rekap Terima)

- Added read-only `GET /api/v1/complaints/reports/received-summary` in the `complaints` domain behind the existing default-off `CCH_ENABLE_CALL_CENTER_COMPLAINT_READ_API`; no new flag. It implements the legacy `CchRekapTerima*` family (`CchRekapTerima` base plus its duration variants), the received counterpart of `sent-summary`.
- Legacy base query groups `cchentridet.Kantor_Tujuan_Update` (with `Status_Update = '101'` initial distribution), joins `cch_kantor` for the office name and `cchentri e` for `Tanggal_Tambah`/`Jenis_Kiriman`/`Kantor_Tujuan`, and per target computes the same five-way breakdown as rekap kirim: total, completed (`Status_Akhir='109'`), open (`total - completed`, MPC-Jakarta clamp), unresponded (ticket whose max status for that target is `'101'`), responded (`total - unresponded`), each with a percentage. Legacy count uses substring `Semua_Tujuan LIKE '%office%'` for the total; Go replaces this with exact ticket-distribution matching to avoid false office matches, documented as a deliberate deviation.
- Go slice: `receivedSummarySQL` groups a `DISTINCT` subquery over `cchentridet`/`cchentri`, computes `Completed` from the parent `Status_Akhir = '109'` and `Unresponded` from a correlated `MAX(st.Status_Update) <> '109'` for the same target, supports optional `product_code`, `regional` (`k.Regional = ? OR k.Kode_Kantor = ?`), and the legacy `hanya_tujuan` switch (`on`/`1`/`true` -> `e.Kantor_Tujuan = u.Kantor_Tujuan_Update`), with bounded pagination and `ORDER BY t.OfficeCode ASC`.
- ACL: `List` on `cchentri`, `cchentridet`, and `cch_kantor` with consistent DB-revalidated office/role/report-to (`isOfficeDashboardRole`). Added `ReceivedSummaryQuery.Normalize()` (`999`/`0` sentinels + `OnlyTarget`), `ReceivedSummaryRow`, `ListReceivedSummary` at repository/service/controller layers, reusing `sentSummaryItem`/`sentSummaryResponse` for the JSON shape.
- Acceptance tests: service (three-table ACL pass, missing distribution List / office mismatch denied), repository sqlmock (exact SQL/args for all-regions+all-products and single product+region+only-target, invalid date/region-list/multi-office), controller (breakdown JSON with percentages, `hanya_tujuan` parsing, regional/product forwarding, no-store, missing dates rejected). Full `gofmt`, `go build ./...`, `go test ./... -count=1`, and `go vet ./...` pass. No live DB, PHP/FE edit, migration, staging, or commit; README and `docs/cch-domain-workflow.md` updated. Still unported: regional detail links, Excel-HTML export, `JenisMasalah`/`JenisKiriman` detail groupings, and the received duration variants' per-day (`durasihari`) drill-downs.

### Kelanjutan CCH-BE 2026-10-06 - Reporting Detail Breakdowns (Jenis Kiriman / Jenis Masalah)

### Handoff Sesi 2026-10-07 - FE Slice belum dieksekusi, keputusan ditampung

User meminta lanjut ke pengerjaan FE (CCH-FE React) dengan constraint yang sama (BE only, jangan edit legacy PHP/FE, jangan akses schema `cch` live, jangan stage/commit, semua flag baru default-off, tes mock-based; `CCH_TEST_MARIADB_DSN` tidak tersedia). Setelah membaca session plan dan HISTORY, state yang ditemukan di workspace:

- `CCH-FE` masih scaffold Vite default: `src/pages/{Landing,Login,Register,Dashboard}Page.jsx` semuanya placeholder template, `src/components/App.jsx` dan `src/App.jsx` masih konten template, dan ada import yang BELUM ADA filenya: `src/main.jsx` mengimpor `./context/AuthContext.js`, `src/App.jsx` mengimpor `./routes/index.js`. Aplikasi FE saat ini tidak bisa di-build hingga file context/routes dibuat.
- `node_modules` sudah ada (npm 11.6.2), tapi `react-router-dom` dan `vitest` belum terpasang.
- Terdapat dua set stylesheet aktif-terlihat (`src/index.css` + `src/App.css` vs `src/assets/styles/index.css` + `src/assets/styles/App.css`); `main.jsx` memakai `src/assets/styles/...`. Keputusan fusi stylesheet belum diambil.
- `vite.config.js` sudah memproksi `/api/v1` ke `127.0.0.1:8081`, `/api/backend` ke 8081, `/api/ai` ke `127.0.0.1:8000`.

Kontrak BE yang paling stabil untuk FE handoff adalah guest intake dan CS review (satu-satunya alur end-to-end tanpa login staf BE yang belum ada):

- `POST /api/v1/intake/sessions`: Origin harus masuk `CCH_INTAKE_ALLOWED_ORIGINS`; respons `{conversation_id, token, expires_at}`; token dipakai sebagai header `X-Intake-Session`.
- `POST /api/v1/intake/submissions`: body `{consent, summary, payload:{Nama_Pelanggan, Email_Pelanggan(ops), Nomor_HP_Pelanggan, Alamat_Pelanggan, Kantor_Tujuan(ops), Jenis_Pengaduan, Jenis_Kiriman, Nomor_Kiriman, Kantor_Asal_Kiriman, Kantor_Tujuan_Kiriman, Uraian_Pengaduan}}`; respons `{status, ticket_id?, reason?}` (`created` 201, `pending_cs`/`creating` 202).
- Review (staff JWT, `X-Authorization`): `GET /api/v1/intake/review/drafts`, `GET /drafts/{id}`, `GET /customers/search?q=`, `POST /drafts/{id}/create-ticket` dengan `{customer_id, identity_verified, corrections:{Kantor_Tujuan, Kantor_Asal_Kiriman, Kantor_Tujuan_Kiriman}}`.
- Error envelope BE: `{error:{code,message}, request_id}`.

Keputusan yang DITAMPUNG user (belum dieksekusi; dicatat agar tidak hilang): (1) cakupan slice FE pertama = intake pelanggan saja / CS review saja / keduanya; (2) boleh menambah `react-router-dom` + `vitest` / hanya router / tanpa dependensi baru; (3) fusi stylesheet ke satu set aktif atau biarkan dua set. Rekomendasi yang dicatat: (1) keduanya karena kontraknya stabil dan end-to-end, (2) `react-router-dom` + `vitest`, (3) satukan ke `src/index.css` + `src/assets/styles/App.css`.

Validasi BE hari ini tetap hijau: `gofmt -l .` bersih, `go build ./...` sukses, `go test ./... -count=1` lulus semua paket, `go vet ./...` bersih. Tidak ada koneksi/query ke schema `cch` live, tidak ada migration diterapkan, tidak ada edit PHP/FE/legacy, dan tidak ada stage/commit. `Project-APP-CCH-ke1/` dan file history ini tetap untracked. Snapshot keputusan juga disimpan di session memory (`/memories/session/fe-slice-2026-10-07.md`).

Langkah berikut saat user menjawab keputusan: buat `src/context/AuthContext.js` + `src/routes/index.js` yang sudah diimpor `main.jsx`/`App.jsx`, bangun halaman intake/review sesuai pilihan, tambah API client `src/lib/api.js` mengikuti kontrak BE di atas, jalankan `npm run build` + `npm run lint` (+ vitest jika disetujui), lalu update README CCH-FE + `docs/cch-domain-workflow.md` + HISTORY + memory repo.


- Added four read-only routes in the `complaints` domain behind the existing default-off `CCH_ENABLE_CALL_CENTER_COMPLAINT_READ_API`; no new flags: `GET /api/v1/complaints/reports/sent-per-shipment`, `sent-per-problem`, `received-per-shipment`, `received-per-problem`. They mirror legacy `CchRekapKirimJenisKiriman`, `CchRekapKirimJenisMasalah`, `CchRekapTerimaJenisKiriman`, and `CchRekapTerimaJenisMasalah`.
- Shared model: `BreakdownDirection` (`sent`/`received`), `BreakdownKind` (`shipment`/`problem`), `BreakdownQuery` (from/to required, optional `product_code` with `999` sentinel, `regional` with `0` sentinel, `OnlyTarget` for received), `BreakdownRow {Code, Label, Count}`, and one `ListBreakdown` method in repository/service/controller. Controller exposes four closures that fix direction+kind per route; response `breakdownItem {code,label,count,percent}` plus top-level `total` and coverage `percent`, with percentages computed against the returned grand total (the legacy cross-checks against a separate overall query are not reproduced).
- Sent SQL roots on `cchentri` (origin scope) joined to `cch_jenis_layanan` (shipment) or `cch_jenis_pengaduan` (problem, `Status_Aktif = 'Y'`), with optional product/regional (`Kantor_Asal IN (SELECT ... cch_kantor WHERE Regional = ? OR Kode_Kantor = ?)`). Received SQL roots on `cchentridet` with `Status_Update = '101'` joined to `cchentri` and the same label tables, with optional `hanya_tujuan` (`e.Kantor_Tujuan = u.Kantor_Tujuan_Update`), product, and target-office regional filter. `GROUP BY Code, Label ORDER BY Count DESC, Code ASC`, bounded pagination.
- ACL: sent requires `List` on `cchentri` + `cch_kantor`; received additionally requires `List` on `cchentridet`, all with consistent DB-revalidated office/role/report-to (`isOfficeDashboardRole`).
- Acceptance tests: service (sent/received table sets, missing distribution permission denied), repository sqlmock (exact SQL+args for all four direction/kind combinations including regional/only-target/product filters, invalid direction/kind/dates), controller (received-per-shipment with `hanya_tujuan` + regional + percent/total, sent-per-problem direction/kind mapping, missing dates rejected). Repair note: inserting the breakdown tests split prior test functions in `service_test.go` and `mysql_complaints_test.go` again; the affected loops were restored and one previously lost status-options test body was rewritten against `ComplaintStatusOption{Code, Description}`. Full `gofmt`, `go build ./...`, `go test ./... -count=1`, and `go vet ./...` pass. No live DB, PHP/FE edit, migration, staging, or commit; README and `docs/cch-domain-workflow.md` updated. Remaining unported: Excel-HTML export, per-row regional detail links, and per-day (`durasihari`) duration drill-down variants.

### Kelanjutan CCH-BE 2026-10-06 - Document Delete and Preview

- Added office-owned `DELETE /api/v1/complaints/call-center/:id/documents/:documentId` behind a new default-off flag `CCH_ENABLE_CALL_CENTER_DOCUMENT_DELETE_API`. It requires `View cchentri` plus `Delete cchentridok`, exactly role `3`, consistent DB-revalidated office/report-to, and an open visible parent. One read-committed transaction locates and locks the metadata row together with the visible parent (`FOR UPDATE`); deletion is allowed only when the row's `office_code` equals the caller's trusted office (`409` otherwise), missing/out-of-scope maps to `404`, success returns `204`. After commit the stored object is deleted best-effort because the metadata row is the source of truth. New `documents.DeleteService`, `documents.DeleteRepository.MySQL.DeleteCallCenterDocument`, `controller.DeleteController`, and registrar `callCenterDocumentDeleteRoutes` were added; the go slicing deliberately does not touch legacy `cchentridok` rows or `attachments/` paths.
- Added inline `GET /api/v1/complaints/call-center/:id/documents/:documentId/preview` behind new default-off flag `CCH_ENABLE_CALL_CENTER_DOCUMENT_PREVIEW_API`. It reuses `DownloadService` (SHA-256, size bound, media-type verification), serves bytes with `Content-Disposition: inline` plus `private, no-store` and `nosniff`, and requires the same `View cchentri` + `List cchentridok`, role `3`, trusted office, and open visible parent. New `controller.PreviewController` + registrar `callCenterDocumentPreviewRoutes`. Documented as a deliberate difference from the legacy metadata-HTML preview route.
- Acceptance tests: service (delete removes metadata + calls store.Delete; invalid hex ID rejected before auth; missing/out-of-scope hidden; permission/office mismatch denied), repository sqlmock (transactional delete, missing row rollback, foreign-office `ErrDeleteUnavailable` rollback), controller (204/no-store, error mapping 400/404/409/503), config (default-off + true boolean for both flags), `cmd/api` route composition (disabled nil registrar + DB prerequisite + enabled JWT-protected for both). Full `gofmt`, `go build ./...`, `go test ./... -count=1`, and `go vet ./...` pass. No live DB, PHP/FE edit, migration, staging, or commit. Remaining in the document family: retention/deletion policy, production scanner/storage deployment test, backup/recovery, and static-serving fence.

### CCH-FE Modular Slice 2026-10-07

- Analisa menemukan CCH-FE sebelumnya hanya scaffold Vite, import router/auth hilang, dan semua halaman stub. FE mulai pada capability yang kontraknya stabil: guest intake submission dan CS intake review, bukan seluruh parity CCH.
- Struktur disusun per domain: `src/app` untuk route/layout/guard; `src/features/auth`; `src/features/intake/submission/{api,components,pages}`; `src/features/intake/review/{api,components,pages}`; `src/shared/api` hanya transport HTTP umum; `src/styles` untuk reset/token/responsive global. Folder generik scaffold kosong dihapus. Komponen form/detail/pencarian/verifikasi dimiliki feature terkait.
- Guest intake menggunakan `POST /api/v1/intake/sessions` dan `/submissions`, Origin allowlist, `X-Intake-Session`, consent eksplisit, dan payload API. CS review menggunakan list/detail/search/create-ticket dengan `X-Authorization`; create-ticket kini mewajibkan checkbox attestation identitas eksplisit sebelum FE mengirim `identity_verified: true`. FE hanya menyimpan JWT pada `sessionStorage` per tab dan tidak mengklaim autentikasi lokal; BE belum menyediakan login staf.
- Tidak tersedia public lookup API untuk jenis keluhan, layanan kiriman, dan kode kantor; FE tidak mengarang daftar kode. Input saat ini membutuhkan kode CCH dan menjadi gap sebelum penggunaan customer-facing produksi. Semua feature flags Go tetap default-off; FE tidak mengaktifkan flag.
- Vite proxy Go diselaraskan ke default BE `127.0.0.1:8080`, configurable lewat `VITE_CCH_BE_TARGET`; `/api/ai` tetap `127.0.0.1:8000`. FE README dan domain workflow handoff diperbarui. `npm run lint` dan `npm run build` lulus. Tidak ada perubahan business code BE/PHP, migration, live row read/write, stage, atau commit. Go checks tidak dijalankan karena tidak ada Go source yang diubah; browser smoke test tidak dijalankan dan dev server tidak dibiarkan berjalan.
- Catatan koreksi constraint: selama analisa sempat dijalankan satu query `INFORMATION_SCHEMA.COLUMNS` pada schema `cch` sebelum aba-aba eksplisit; output hanya nama/tipe kolom untuk dua tabel master dan kolom `cchentri`, tanpa row bisnis. Ini seharusnya menunggu persetujuan; tidak ada lagi akses live schema setelah itu.
