package payment_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
	usecase "github.com/Pravasta/payment-service/internal/usecase/payment"
)

// --- mock Repository ---

type mockRepo struct {
	byID      map[uuid.UUID]*domain.Transaction
	byIdem    map[string]*domain.Transaction
	byExtRef  map[string]*domain.Transaction
	events    []*domain.TransactionEvent
	createErr error
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

func (m *mockRepo) AppendEvent(_ context.Context, e *domain.TransactionEvent) error {
	m.events = append(m.events, e)
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
	calls     int
	result    domain.ChargeResult
	chargeErr error
}

func (g *mockGateway) CreateCharge(_ context.Context, _ domain.ChargeRequest) (domain.ChargeResult, error) {
	g.calls++
	if g.chargeErr != nil {
		return domain.ChargeResult{}, g.chargeErr
	}
	return g.result, nil
}

func (g *mockGateway) ParseWebhook(_ context.Context, _ domain.WebhookPayload) (domain.WebhookEvent, error) {
	return domain.WebhookEvent{}, errors.New("not impl")
}
func (g *mockGateway) GetStatus(_ context.Context, _ domain.StatusRef) (domain.StatusResult, error) {
	return domain.StatusResult{}, errors.New("not impl")
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
