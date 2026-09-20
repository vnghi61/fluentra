package service

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/payment/contract"
	"github.com/fluentra/fluentra/internal/modules/payment/domain"
	"github.com/fluentra/fluentra/internal/modules/payment/repository"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
)

var fluReferenceRegex = regexp.MustCompile(`(?i)FLU[A-Za-z0-9]{10}`)

// Config holds configuration for SePay payment processing.
type Config struct {
	WebhookAPIKey string
	APIToken      string
	AccountNumber string
	BankCode      string
	AccountHolder string
	AllowedIPs    string
	OrderTTL      time.Duration
}

// Service orchestrates billing orders, SePay webhook ingestion, matching, and reconciliation.
type Service interface {
	CreateOrder(ctx context.Context, in contract.CreateOrderInput) (*domain.Order, error)
	GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error)
	HandleSepayWebhook(ctx context.Context, authHeader, clientIP string, rawBody []byte, payload *domain.SepayWebhookPayload) error
	MatchTransaction(ctx context.Context, tx *domain.SepayTransaction) error
	SweepExpiredOrders(ctx context.Context) (int, error)
	Reconcile(ctx context.Context) error
	ListUnmatchedTransactions(ctx context.Context, limit, offset int32) (*domain.UnmatchedTransactionsList, error)

	// RecordRefund records that money is owed back on a paid order, and moves
	// the order to refunded. It does not move money: SePay only receives.
	RecordRefund(
		ctx context.Context, orderID uuid.UUID, amountVND int64, reason string, actorID uuid.UUID,
	) (*domain.Refund, error)
	ListRefunds(ctx context.Context, status *string, limit, offset int) ([]domain.Refund, int64, error)
	MarkRefundSent(ctx context.Context, id uuid.UUID) (*domain.Refund, error)

	CreatePayout(ctx context.Context, in contract.CreatePayoutInput) (*domain.Payout, error)
	GetPayout(ctx context.Context, id uuid.UUID) (*domain.Payout, error)
	ListPayouts(ctx context.Context, status *string, limit, offset int) ([]domain.Payout, int64, error)
	ListCreatorPayouts(ctx context.Context, creatorID uuid.UUID, limit, offset int) ([]domain.Payout, error)
	GetPendingPayoutTotal(ctx context.Context, creatorID uuid.UUID) (int64, error)
	FulfillPayout(ctx context.Context, id uuid.UUID, bankReference string, actorID uuid.UUID) (*domain.Payout, error)
}

type paymentService struct {
	repo       repository.Repository
	cfg        Config
	httpClient *http.Client
	bus        eventbus.EventBus
}

// NewService creates a new payment service.
func NewService(repo repository.Repository, cfg Config, bus ...eventbus.EventBus) Service {
	if cfg.OrderTTL <= 0 {
		cfg.OrderTTL = 24 * time.Hour
	}
	var eb eventbus.EventBus
	if len(bus) > 0 {
		eb = bus[0]
	}
	return &paymentService{
		repo: repo,
		cfg:  cfg,
		bus:  eb,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *paymentService) CreateOrder(ctx context.Context, in contract.CreateOrderInput) (*domain.Order, error) {
	if in.AmountVND <= 0 {
		return nil, domain.ErrInvalidAmount
	}
	orderID := uuid.New()
	ref := domain.GenerateReference(orderID)
	now := time.Now().UTC()
	expiresAt := now.Add(s.cfg.OrderTTL)
	qrURL := domain.BuildVietQRURL(s.cfg.BankCode, s.cfg.AccountNumber, in.AmountVND, ref)

	order := &domain.Order{
		ID:                orderID,
		UserID:            in.UserID,
		Reference:         ref,
		AmountVND:         in.AmountVND,
		Status:            domain.OrderStatusPending,
		SubjectKind:       in.SubjectKind,
		SubjectID:         in.SubjectID,
		QRURL:             qrURL,
		BankCode:          s.cfg.BankCode,
		AccountNumber:     s.cfg.AccountNumber,
		AccountHolderName: s.cfg.AccountHolder,
		ExpiresAt:         expiresAt,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	created, err := s.repo.CreateOrder(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	// Ensure QR and bank metadata are populated in response
	created.QRURL = qrURL
	created.BankCode = s.cfg.BankCode
	created.AccountNumber = s.cfg.AccountNumber
	created.AccountHolderName = s.cfg.AccountHolder
	return created, nil
}

func (s *paymentService) GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	order, err := s.repo.GetOrderByID(ctx, id)
	if err != nil {
		return nil, err
	}
	order.QRURL = domain.BuildVietQRURL(s.cfg.BankCode, s.cfg.AccountNumber, order.AmountVND, order.Reference)
	order.BankCode = s.cfg.BankCode
	order.AccountNumber = s.cfg.AccountNumber
	order.AccountHolderName = s.cfg.AccountHolder
	return order, nil
}

func (s *paymentService) HandleSepayWebhook(ctx context.Context, authHeader, clientIP string, rawBody []byte, payload *domain.SepayWebhookPayload) error {
	// 1. Verify API Key
	if strings.TrimSpace(s.cfg.WebhookAPIKey) == "" {
		return domain.ErrInvalidWebhookKey
	}

	var token string
	lowerAuth := strings.ToLower(strings.TrimSpace(authHeader))
	if strings.HasPrefix(lowerAuth, "apikey ") {
		token = strings.TrimSpace(authHeader[7:])
	} else if strings.HasPrefix(lowerAuth, "bearer ") {
		token = strings.TrimSpace(authHeader[7:])
	} else {
		token = strings.TrimSpace(authHeader)
	}

	if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.WebhookAPIKey)) != 1 {
		return domain.ErrInvalidWebhookKey
	}

	// Verify allowed IPs if configured
	if strings.TrimSpace(s.cfg.AllowedIPs) != "" {
		allowed := false
		clientIPTrimmed := strings.TrimSpace(clientIP)
		for _, ip := range strings.Split(s.cfg.AllowedIPs, ",") {
			if strings.TrimSpace(ip) == clientIPTrimmed {
				allowed = true
				break
			}
		}
		if !allowed {
			return domain.ErrForbiddenSenderIP
		}
	}

	// 2. Store raw webhook in billing.payment_webhooks (ON CONFLICT DO NOTHING).
	//
	// BR-PAYMENT-05 keeps every webhook so it can be replayed after a bug fix
	// without asking SePay to resend. A failure to store one is logged rather
	// than failing the request: SePay retries a non-200 for five hours, and a
	// transaction we can process is worth more than the copy of its envelope.
	sepayIDStr := strconv.FormatInt(payload.ID, 10)
	if err := s.repo.InsertPaymentWebhook(ctx, "sepay", sepayIDStr, rawBody, true, nil); err != nil {
		slog.ErrorContext(ctx, "could not store raw sepay webhook; replay will not be possible for it",
			"sepay_id", payload.ID, "error", err)
	}

	// 3. Parse transaction date
	txDate, err := time.Parse("2006-01-02 15:04:05", payload.TransactionDate)
	if err != nil {
		txDate = time.Now().UTC()
	}

	// 4. Insert into billing.sepay_transactions
	tx := &domain.SepayTransaction{
		SepayID:         payload.ID,
		Gateway:         payload.Gateway,
		TransactionDate: txDate,
		AccountNumber:   payload.AccountNumber,
		SubAccount:      payload.SubAccount,
		Code:            payload.Code,
		Content:         payload.Content,
		TransferType:    payload.TransferType,
		TransferAmount:  payload.TransferAmount,
		ReferenceCode:   payload.ReferenceCode,
		Accumulated:     payload.Accumulated,
	}

	inserted, err := s.repo.InsertSepayTransaction(ctx, tx)
	if err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}

	// 5. Run matching logic
	if inserted != nil {
		_ = s.MatchTransaction(ctx, inserted)
	}

	return nil
}

func (s *paymentService) MatchTransaction(ctx context.Context, tx *domain.SepayTransaction) error {
	// If already matched, skip
	if tx.MatchedAt != nil && tx.OrderID != nil {
		return nil
	}

	// Rule 1: transferType must be 'in'. Ignore 'out' transactions.
	if strings.ToLower(tx.TransferType) != "in" {
		reason := "transfer_type_out"
		_, err := s.repo.UpdateSepayTransactionMatch(ctx, tx.ID, nil, nil, &reason)
		return err
	}

	// Rule 2: Extract reference from content
	cleanContent := domain.StripNonAlphanumeric(tx.Content)
	candidates := fluReferenceRegex.FindAllString(cleanContent, -1)
	if len(candidates) == 0 {
		candidates = fluReferenceRegex.FindAllString(strings.ToUpper(tx.Content), -1)
	}

	var matchedOrder *domain.Order
	for _, candidate := range candidates {
		order, err := s.repo.GetOrderByReference(ctx, strings.ToUpper(candidate))
		if err == nil && order != nil {
			if domain.ContentMatchesReference(tx.Content, order.Reference) {
				matchedOrder = order
				break
			}
		}
	}

	if matchedOrder == nil {
		reason := "order_not_found"
		_, err := s.repo.UpdateSepayTransactionMatch(ctx, tx.ID, nil, nil, &reason)
		return err
	}

	// Rule 3: transferAmount must equal amount_vnd exactly
	if tx.TransferAmount != matchedOrder.AmountVND {
		reason := fmt.Sprintf("amount_mismatch: got %d want %d", tx.TransferAmount, matchedOrder.AmountVND)
		_, err := s.repo.UpdateSepayTransactionMatch(ctx, tx.ID, nil, nil, &reason)
		return err
	}

	// Rule 4: Match succeeded.
	//
	// An order lives 24 hours. A payment arriving against an expired order still
	// matches — the money is real — and MarkOrderPaid accepts `expired` for that
	// reason, rather than leaving the learner paid and unenrolled.
	//
	// It refuses an order that is already paid, cancelled or refunded, and that
	// refusal is the point: a learner who transfers twice because they thought
	// the first attempt failed produces a second transaction with the same
	// reference and the same amount. Marking the order paid again republished
	// payment.succeeded and filed the transaction as matched, so their second
	// payment left no trace anyone would ever look at. It now lands in the
	// unmatched queue, which is where a human can see it and refund it.
	now := time.Now().UTC()
	paidOrder, err := s.repo.MarkOrderPaid(ctx, matchedOrder.ID, now)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotPayable) {
			reason := fmt.Sprintf("duplicate_payment: order %s is already %s", matchedOrder.ID, matchedOrder.Status)
			slog.WarnContext(ctx, "payment received against an order that was not payable",
				"order_id", matchedOrder.ID, "order_status", matchedOrder.Status,
				"sepay_id", tx.SepayID, "amount_vnd", tx.TransferAmount)
			_, updErr := s.repo.UpdateSepayTransactionMatch(ctx, tx.ID, nil, nil, &reason)
			return updErr
		}
		return fmt.Errorf("update order status paid: %w", err)
	}
	_ = paidOrder

	_, err = s.repo.UpdateSepayTransactionMatch(ctx, tx.ID, &matchedOrder.ID, &now, nil)
	if err != nil {
		return fmt.Errorf("update transaction matched: %w", err)
	}

	slog.InfoContext(ctx, "payment matched successfully",
		"order_id", matchedOrder.ID,
		"reference", matchedOrder.Reference,
		"amount_vnd", tx.TransferAmount,
		"sepay_id", tx.SepayID,
	)

	// Publish payment.succeeded event
	if s.bus != nil {
		payload, err := json.Marshal(contract.EventPaymentSucceeded{
			UserID:      matchedOrder.UserID,
			OrderID:     matchedOrder.ID,
			SubjectKind: matchedOrder.SubjectKind,
			SubjectID:   matchedOrder.SubjectID,
			AmountVND:   matchedOrder.AmountVND,
		})
		if err == nil {
			_ = s.bus.Publish(ctx, eventbus.Message{
				ID:      uuid.New(),
				Topic:   "payment.succeeded",
				Payload: payload,
			})
		}
	}

	return nil
}

func (s *paymentService) SweepExpiredOrders(ctx context.Context) (int, error) {
	orders, err := s.repo.ListExpiredPendingOrders(ctx, 100)
	if err != nil {
		return 0, fmt.Errorf("list expired orders: %w", err)
	}

	count := 0
	for _, o := range orders {
		_, err := s.repo.UpdateOrderStatus(ctx, o.ID, domain.OrderStatusExpired, nil)
		if err == nil {
			count++
		}
	}
	return count, nil
}

// SePayAPIResponse represents the response envelope from SePay userapi transactions list.
type SePayAPIResponse struct {
	Status       int                `json:"status"`
	Error        *string            `json:"error"`
	Transactions []SePayTransaction `json:"transactions"`
}

// SePayTransaction represents a transaction record in the SePay API list response.
//
// Every numeric field is a JSON *string* in this API — `"id": "49682"`,
// `"amount_in": "18067000.00"` — unlike the webhook, which sends numbers. They
// were declared as int64 and float64, so the response failed to decode on its
// first field and Reconcile returned an error on every run without ever
// processing a transaction. Since reconciliation is what catches a webhook that
// never arrived (BR-PAYMENT-07), the safety net was not attached to anything.
//
// The amounts are read as strings and parsed as whole VND. The float they used
// to be parsed into was the other half of the problem: VND has no subunit, and
// money does not go through float64 in this module (BR-PAYMENT-10).
type SePayTransaction struct {
	ID                 string `json:"id"`
	AmountIn           string `json:"amount_in"`
	AmountOut          string `json:"amount_out"`
	TransactionContent string `json:"transaction_content"`
	ReferenceNumber    string `json:"reference_number"`
	AccountNumber      string `json:"account_number"`
	SubAccount         string `json:"sub_account"`
	TransactionDate    string `json:"transaction_date"`
	BankBrandName      string `json:"bank_brand_name"`
}

// parseVND reads SePay's "18067000.00" as 18067000 whole dong.
//
// The fractional part is always zero because VND has no subunit; it is dropped
// rather than rounded, and a value that is not a number at all is an error
// rather than a silent zero, because a zero here is a transaction that looks
// like it moved no money.
func parseVND(raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	if dot := strings.IndexByte(trimmed, '.'); dot >= 0 {
		trimmed = trimmed[:dot]
	}
	if trimmed == "" || trimmed == "-" {
		return 0, nil
	}
	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse amount %q: %w", raw, err)
	}
	return value, nil
}

func (s *paymentService) Reconcile(ctx context.Context) error {
	if strings.TrimSpace(s.cfg.APIToken) == "" {
		slog.DebugContext(ctx, "sepay api token not set; skipping reconciliation")
		return nil
	}

	lastSeenID, err := s.repo.GetLastSeenSepayID(ctx)
	if err != nil {
		return fmt.Errorf("get last seen sepay id: %w", err)
	}

	url := fmt.Sprintf("https://my.sepay.vn/userapi/transactions/list?since_id=%d&limit=5000", lastSeenID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build reconcile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.APIToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do reconcile request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get("x-sepay-userapi-retry-after")
		slog.WarnContext(ctx, "sepay api rate limit hit during reconciliation", "retry_after", retryAfter)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sepay api returned status %d", resp.StatusCode)
	}

	var apiResp SePayAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("decode sepay reconcile response: %w", err)
	}

	for _, item := range apiResp.Transactions {
		txDate, parseErr := time.Parse("2006-01-02 15:04:05", item.TransactionDate)
		if parseErr != nil {
			txDate = time.Now().UTC()
		}

		sepayID, idErr := strconv.ParseInt(strings.TrimSpace(item.ID), 10, 64)
		if idErr != nil {
			slog.WarnContext(ctx, "skipping sepay transaction with unparseable id", "id", item.ID, "error", idErr)
			continue
		}
		amountIn, inErr := parseVND(item.AmountIn)
		amountOut, outErr := parseVND(item.AmountOut)
		if inErr != nil || outErr != nil {
			slog.WarnContext(ctx, "skipping sepay transaction with unparseable amount",
				"sepay_id", sepayID, "amount_in", item.AmountIn, "amount_out", item.AmountOut)
			continue
		}

		transferType := "in"
		transferAmount := amountIn
		if amountOut > 0 {
			transferType = "out"
			transferAmount = amountOut
		}

		tx := &domain.SepayTransaction{
			SepayID:         sepayID,
			Gateway:         item.BankBrandName,
			TransactionDate: txDate,
			AccountNumber:   item.AccountNumber,
			SubAccount:      item.SubAccount,
			Content:         item.TransactionContent,
			TransferType:    transferType,
			TransferAmount:  transferAmount,
			ReferenceCode:   item.ReferenceNumber,
		}

		inserted, err := s.repo.InsertSepayTransaction(ctx, tx)
		if err == nil && inserted != nil {
			_ = s.MatchTransaction(ctx, inserted)
		}
	}

	return nil
}

func (s *paymentService) ListUnmatchedTransactions(ctx context.Context, limit, offset int32) (*domain.UnmatchedTransactionsList, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	items, total, err := s.repo.ListUnmatchedTransactions(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list unmatched transactions: %w", err)
	}
	return &domain.UnmatchedTransactionsList{
		Items: items,
		Total: total,
	}, nil
}

// RecordRefund writes the obligation first and moves the order second.
//
// In that order on purpose: a refund row with an order still marked paid is a
// visible inconsistency an admin can resolve, while an order marked refunded
// with no row is money owed that nobody will ever be told about.
func (s *paymentService) RecordRefund(
	ctx context.Context, orderID uuid.UUID, amountVND int64, reason string, actorID uuid.UUID,
) (*domain.Refund, error) {
	if amountVND <= 0 {
		return nil, domain.ErrInvalidAmount
	}
	refund, err := s.repo.CreateRefund(ctx, orderID, amountVND, reason, actorID)
	if err != nil {
		return nil, fmt.Errorf("record refund: %w", err)
	}
	if _, err := s.repo.MarkOrderRefunded(ctx, orderID); err != nil {
		// The obligation is recorded, which is the part that matters. An order
		// that did not move is reported so the caller can refuse to revoke.
		return nil, fmt.Errorf("mark order refunded: %w", err)
	}
	slog.InfoContext(ctx, "refund recorded",
		"order_id", orderID, "amount_vnd", amountVND, "refund_id", refund.ID)
	return refund, nil
}

func (s *paymentService) ListRefunds(
	ctx context.Context, status *string, limit, offset int,
) ([]domain.Refund, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListRefunds(ctx, status, int32(limit), int32(offset))
}

func (s *paymentService) MarkRefundSent(ctx context.Context, id uuid.UUID) (*domain.Refund, error) {
	return s.repo.MarkRefundSent(ctx, id)
}

func (s *paymentService) CreatePayout(ctx context.Context, in contract.CreatePayoutInput) (*domain.Payout, error) {
	if in.AmountVND <= 0 {
		return nil, domain.ErrInvalidAmount
	}
	p := &domain.Payout{
		CreatorID: in.CreatorID,
		AmountVND: in.AmountVND,
		ActorID:   in.ActorID,
	}
	return s.repo.CreatePayout(ctx, p)
}

func (s *paymentService) GetPayout(ctx context.Context, id uuid.UUID) (*domain.Payout, error) {
	return s.repo.GetPayoutByID(ctx, id)
}

func (s *paymentService) ListPayouts(
	ctx context.Context,
	status *string,
	limit, offset int,
) ([]domain.Payout, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListPayouts(ctx, status, int32(limit), int32(offset))
}

func (s *paymentService) ListCreatorPayouts(
	ctx context.Context,
	creatorID uuid.UUID,
	limit, offset int,
) ([]domain.Payout, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListPayoutsByCreatorID(ctx, creatorID, int32(limit), int32(offset))
}

func (s *paymentService) GetPendingPayoutTotal(ctx context.Context, creatorID uuid.UUID) (int64, error) {
	return s.repo.GetPendingPayoutTotalByCreatorID(ctx, creatorID)
}

func (s *paymentService) FulfillPayout(
	ctx context.Context,
	id uuid.UUID,
	bankReference string,
	actorID uuid.UUID,
) (*domain.Payout, error) {
	payout, err := s.repo.GetPayoutByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if payout.Status != domain.PayoutStatusPending {
		return nil, domain.ErrPayoutAlreadyProcessed
	}
	now := time.Now().UTC()
	updated, err := s.repo.UpdatePayoutStatus(ctx, id, domain.PayoutStatusSent, &bankReference, actorID, &now)
	if err != nil {
		return nil, err
	}

	if s.bus != nil {
		payload, err := json.Marshal(contract.EventPayoutSent{
			PayoutID:      updated.ID,
			CreatorID:     updated.CreatorID,
			AmountVND:     updated.AmountVND,
			BankReference: bankReference,
		})
		if err == nil {
			_ = s.bus.Publish(ctx, eventbus.Message{
				ID:      uuid.New(),
				Topic:   "payment.payout_sent",
				Payload: payload,
			})
		}
	}

	return updated, nil
}
