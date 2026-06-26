package payment_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
	usecase "github.com/Pravasta/payment-service/internal/usecase/payment"
)

// --- mock Repository ---

type mockRepo struct {
	byID       map[uuid.UUID]*domain.Transaction
	byIdem     map[string]*domain.Transaction
	byExtRef   map[string]*domain.Transaction
	events     []*domain.TransactionEvent
	outbox     []*domain.OutboxMessage
	inboxSaved int
	createErr  error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		byID:     map[uuid.UUID]*domain.Transaction{},
		byIdem:   map[string]*domain.Transaction{},
		byExtRef: map[string]*domain.Transaction{},
	}
}

func key(appID uuid.UUID, s string) string { return appID.String() + "|" + s }

func (m *mockRepo) Create(_ context.Context, txn *domain.Transaction) error {
	if m.createErr != nil {
		return m.createErr
	}
	cp := *txn
	m.byID[txn.ID] = &cp
	if txn.IdempotencyKey != "" {
		m.byIdem[key(txn.AppID, txn.IdempotencyKey)] = &cp
	}
	m.byExtRef[key(txn.AppID, txn.ExternalReference)] = &cp
	return nil
}

func (m *mockRepo) Update(_ context.Context, txn *domain.Transaction) error {
	cp := *txn
	m.byID[txn.ID] = &cp
	if txn.IdempotencyKey != "" {
		m.byIdem[key(txn.AppID, txn.IdempotencyKey)] = &cp
	}
	m.byExtRef[key(txn.AppID, txn.ExternalReference)] = &cp
	return nil
}

func (m *mockRepo) GetByID(_ context.Context, appID, id uuid.UUID) (*domain.Transaction, error) {
	if t, ok := m.byID[id]; ok && t.AppID == appID {
		return t, nil
	}
	return nil, domain.ErrNotFound
}

func (m *mockRepo) GetByExternalReference(_ context.Context, appID uuid.UUID, ref string) (*domain.Transaction, error) {
	if t, ok := m.byExtRef[key(appID, ref)]; ok {
		return t, nil
	}
	return nil, domain.ErrNotFound
}

func (m *mockRepo) GetByIdempotencyKey(_ context.Context, appID uuid.UUID, k string) (*domain.Transaction, error) {
	if t, ok := m.byIdem[key(appID, k)]; ok {
		return t, nil
	}
	return nil, domain.ErrNotFound
}

func (m *mockRepo) GetByGatewayTxnID(_ context.Context, _, _ string) (*domain.Transaction, error) {
	return nil, domain.ErrNotFound
}

func (m *mockRepo) ListPendingForReconcile(_ context.Context, olderThan time.Time, limit int) ([]*domain.Transaction, error) {
	var out []*domain.Transaction
	for _, t := range m.byID {
		if t.Status == domain.StatusPending && !t.CreatedAt.After(olderThan) {
			out = append(out, t)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// listRows dapat di-set test untuk mengontrol hasil ListTransactions.
func (m *mockRepo) ListTransactions(_ context.Context, f domain.ListFilter) ([]*domain.Transaction, error) {
	var out []*domain.Transaction
	for _, t := range m.byID {
		if t.AppID != f.AppID {
			continue
		}
		if f.Status != "" && t.Status != f.Status {
			continue
		}
		out = append(out, t)
	}
	// urut created_at DESC, id DESC (stabil) — meniru query repo.
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID.String() > out[j].ID.String()
	})
	// keyset cursor.
	if f.CursorCreated != nil && f.CursorID != nil {
		filtered := out[:0]
		for _, t := range out {
			if t.CreatedAt.Before(*f.CursorCreated) ||
				(t.CreatedAt.Equal(*f.CursorCreated) && t.ID.String() < f.CursorID.String()) {
				filtered = append(filtered, t)
			}
		}
		out = filtered
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (m *mockRepo) AppendEvent(_ context.Context, e *domain.TransactionEvent) error {
	m.events = append(m.events, e)
	return nil
}

// --- webhook support (issue 0008) ---

func (m *mockRepo) GetByGatewayRequestID(_ context.Context, _, requestID string) (*domain.Transaction, error) {
	for _, t := range m.byID {
		if t.GatewayRequestID == requestID && requestID != "" {
			return t, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockRepo) SaveWebhookInbox(_ context.Context, rec *domain.WebhookInboxRecord) error {
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}
	m.inboxSaved++
	return nil
}

func (m *mockRepo) UpdateWebhookInbox(_ context.Context, _ *domain.WebhookInboxRecord) error {
	return nil
}

func (m *mockRepo) EventExists(_ context.Context, gatewayEventID string) (bool, error) {
	if gatewayEventID == "" {
		return false, nil
	}
	for _, e := range m.events {
		if e.GatewayEventID == gatewayEventID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockRepo) ApplyWebhook(_ context.Context, txn *domain.Transaction, event *domain.TransactionEvent, outbox *domain.OutboxMessage) error {
	cp := *txn
	m.byID[txn.ID] = &cp
	m.events = append(m.events, event)
	if outbox != nil {
		m.outbox = append(m.outbox, outbox)
	}
	return nil
}

func (m *mockRepo) eventTypes() []domain.EventType {
	var out []domain.EventType
	for _, e := range m.events {
		out = append(out, e.EventType)
	}
	return out
}

// --- mock Gateway ---

type mockGateway struct {
	calls        int
	result       domain.ChargeResult
	chargeErr    error
	webhookEvent domain.WebhookEvent
	webhookErr   error
	statusResult domain.StatusResult
	statusErr    error
	statusCalls  int
}

func (g *mockGateway) CreateCharge(_ context.Context, _ domain.ChargeRequest) (domain.ChargeResult, error) {
	g.calls++
	if g.chargeErr != nil {
		return domain.ChargeResult{}, g.chargeErr
	}
	return g.result, nil
}

func (g *mockGateway) ParseWebhook(_ context.Context, _ domain.WebhookPayload) (domain.WebhookEvent, error) {
	return g.webhookEvent, g.webhookErr
}
func (g *mockGateway) GetStatus(_ context.Context, _ domain.StatusRef) (domain.StatusResult, error) {
	g.statusCalls++
	return g.statusResult, g.statusErr
}
func (g *mockGateway) Refund(_ context.Context, _ domain.RefundRequest) (domain.RefundResult, error) {
	return domain.RefundResult{}, errors.New("not impl")
}

func okGateway() *mockGateway {
	exp := time.Date(2026, 6, 26, 6, 0, 0, 0, time.UTC)
	return &mockGateway{
		result: domain.ChargeResult{
			GatewayTxnID:     "tok-abc",
			GatewayRequestID: "req-xyz",
			PaymentURL:       "https://checkout.doku.com/link/abc",
			Status:           domain.StatusPending,
			ExpiresAt:        &exp,
		},
	}
}

func validInput(appID uuid.UUID) usecase.CreatePaymentInput {
	return usecase.CreatePaymentInput{
		AppID:             appID,
		IdempotencyKey:    "idem-1",
		ExternalReference: "INV-2026-001",
		AmountMinor:       150000,
		Currency:          "IDR",
		ExpiryMinutes:     60,
		ReturnURL:         "https://app.test/return",
	}
}

// --- tests ---

func TestCreatePayment_Success(t *testing.T) {
	repo := newMockRepo()
	gw := okGateway()
	svc := usecase.NewService(repo, nil, gw)
	appID := uuid.New()

	txn, err := svc.CreatePayment(context.Background(), validInput(appID))
	if err != nil {
		t.Fatalf("CreatePayment error: %v", err)
	}

	if txn.Status != domain.StatusPending {
		t.Errorf("status = %q, ingin pending", txn.Status)
	}
	if txn.PaymentURL != "https://checkout.doku.com/link/abc" {
		t.Errorf("payment_url = %q", txn.PaymentURL)
	}
	if txn.GatewayRequestID != "req-xyz" {
		t.Errorf("gateway_request_id = %q", txn.GatewayRequestID)
	}
	if txn.ExpiresAt == nil {
		t.Error("expires_at nil")
	}
	if txn.CurrencyExponent != 0 {
		t.Errorf("currency_exponent = %d, ingin 0 (IDR)", txn.CurrencyExponent)
	}
	if gw.calls != 1 {
		t.Errorf("gateway dipanggil %d kali, ingin 1", gw.calls)
	}

	// Event: payment.created lalu payment.pending.
	ets := repo.eventTypes()
	if len(ets) != 2 || ets[0] != domain.EventCreated || ets[1] != domain.EventPending {
		t.Errorf("events = %v, ingin [created pending]", ets)
	}

	// Row tersimpan & dapat dibaca kembali.
	if _, err := repo.GetByID(context.Background(), appID, txn.ID); err != nil {
		t.Errorf("transaksi tidak tersimpan: %v", err)
	}
}

func TestCreatePayment_IdempotentRetry(t *testing.T) {
	repo := newMockRepo()
	gw := okGateway()
	svc := usecase.NewService(repo, nil, gw)
	appID := uuid.New()

	first, err := svc.CreatePayment(context.Background(), validInput(appID))
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := svc.CreatePayment(context.Background(), validInput(appID))
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("retry menghasilkan id berbeda: %s vs %s", first.ID, second.ID)
	}
	if gw.calls != 1 {
		t.Errorf("gateway dipanggil %d kali, ingin 1 (tidak charge ulang)", gw.calls)
	}
}

func TestCreatePayment_DuplicateExternalReference(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, nil, okGateway())
	appID := uuid.New()

	if _, err := svc.CreatePayment(context.Background(), validInput(appID)); err != nil {
		t.Fatalf("first: %v", err)
	}

	// external_reference sama, idempotency key berbeda → 409 (duplicate).
	in := validInput(appID)
	in.IdempotencyKey = "idem-2"
	_, err := svc.CreatePayment(context.Background(), in)
	if !errors.Is(err, domain.ErrDuplicateReference) {
		t.Fatalf("ingin ErrDuplicateReference, dapat %v", err)
	}
}

func TestCreatePayment_InvalidAmount(t *testing.T) {
	svc := usecase.NewService(newMockRepo(), nil, okGateway())
	in := validInput(uuid.New())
	in.AmountMinor = 0
	if _, err := svc.CreatePayment(context.Background(), in); !errors.Is(err, domain.ErrInvalidAmount) {
		t.Fatalf("ingin ErrInvalidAmount, dapat %v", err)
	}
}

func TestCreatePayment_UnsupportedCurrency(t *testing.T) {
	svc := usecase.NewService(newMockRepo(), nil, okGateway())
	in := validInput(uuid.New())
	in.Currency = "USD"
	if _, err := svc.CreatePayment(context.Background(), in); !errors.Is(err, domain.ErrUnsupportedCurrency) {
		t.Fatalf("ingin ErrUnsupportedCurrency, dapat %v", err)
	}
}

func TestGetPayment_Isolation(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, nil, okGateway())
	appA, appB := uuid.New(), uuid.New()

	txn, err := svc.CreatePayment(context.Background(), validInput(appA))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// app pemilik bisa baca.
	if _, err := svc.GetPayment(context.Background(), appA, txn.ID); err != nil {
		t.Errorf("pemilik tidak bisa baca: %v", err)
	}
	// app lain → not found (isolasi per app_id).
	if _, err := svc.GetPayment(context.Background(), appB, txn.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("app lain: ingin ErrNotFound, dapat %v", err)
	}
}

func TestListPayments_ScopedAndPaginated(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, nil, okGateway())
	appA, appB := uuid.New(), uuid.New()

	// Sisipkan 3 transaksi appA dengan created_at menurun, 1 transaksi appB.
	base := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	for i := range 3 {
		_ = repo.Create(context.Background(), &domain.Transaction{
			ID: uuid.New(), AppID: appA, ExternalReference: "A-" + string(rune('0'+i)),
			Status: domain.StatusPending, CreatedAt: base.Add(time.Duration(i) * time.Minute),
		})
	}
	_ = repo.Create(context.Background(), &domain.Transaction{
		ID: uuid.New(), AppID: appB, ExternalReference: "B-0",
		Status: domain.StatusPending, CreatedAt: base,
	})

	// Halaman 1: limit 2 → 2 item, HasMore true.
	page1, err := svc.ListPayments(context.Background(), usecase.ListPaymentsInput{AppID: appA, Limit: 2})
	if err != nil {
		t.Fatalf("list page1: %v", err)
	}
	if len(page1.Items) != 2 {
		t.Fatalf("page1 items = %d, ingin 2", len(page1.Items))
	}
	if !page1.HasMore {
		t.Error("page1 HasMore harus true")
	}
	if page1.NextCursorID == nil {
		t.Fatal("page1 NextCursor nil")
	}

	// Halaman 2: pakai cursor → sisa 1 item, HasMore false.
	page2, err := svc.ListPayments(context.Background(), usecase.ListPaymentsInput{
		AppID: appA, Limit: 2,
		CursorCreated: page1.NextCursorCreated, CursorID: page1.NextCursorID,
	})
	if err != nil {
		t.Fatalf("list page2: %v", err)
	}
	if len(page2.Items) != 1 {
		t.Errorf("page2 items = %d, ingin 1", len(page2.Items))
	}
	if page2.HasMore {
		t.Error("page2 HasMore harus false")
	}

	// Scoping: appB hanya melihat miliknya.
	listB, _ := svc.ListPayments(context.Background(), usecase.ListPaymentsInput{AppID: appB})
	if len(listB.Items) != 1 {
		t.Errorf("appB items = %d, ingin 1 (scoped)", len(listB.Items))
	}
}

func TestListPayments_LimitClamped(t *testing.T) {
	svc := usecase.NewService(newMockRepo(), nil, okGateway())
	// limit di atas max → tidak error (di-clamp ke maxListLimit secara internal).
	if _, err := svc.ListPayments(context.Background(), usecase.ListPaymentsInput{AppID: uuid.New(), Limit: 9999}); err != nil {
		t.Fatalf("list: %v", err)
	}
}

func TestCreatePayment_GatewayErrorMarksFailed(t *testing.T) {
	repo := newMockRepo()
	gw := &mockGateway{chargeErr: errors.New("doku down")}
	svc := usecase.NewService(repo, nil, gw)
	appID := uuid.New()

	_, err := svc.CreatePayment(context.Background(), validInput(appID))
	if err == nil {
		t.Fatal("ingin error dari gateway")
	}

	// Transaksi harus ada & berstatus failed.
	var stored *domain.Transaction
	for _, txn := range repo.byID {
		stored = txn
	}
	if stored == nil {
		t.Fatal("transaksi tidak tersimpan")
	}
	if stored.Status != domain.StatusFailed {
		t.Errorf("status = %q, ingin failed", stored.Status)
	}
	if stored.FailedAt == nil {
		t.Error("failed_at nil")
	}

	// Event: created lalu failed.
	ets := repo.eventTypes()
	if len(ets) != 2 || ets[0] != domain.EventCreated || ets[1] != domain.EventFailed {
		t.Errorf("events = %v, ingin [created failed]", ets)
	}
}
