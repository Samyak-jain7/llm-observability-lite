package billing

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/billingmeter"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/subcription"
	"github.com/stripe/stripe-go/v76/webhook"
)

var (
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrInvalidSignature     = errors.New("invalid stripe webhook signature")
)

// Plan limits (traces per month).
var PlanLimits = map[string]int64{
	"free":     10_000,
	"dev":      100_000,
	"startup":  1_000_000,
	"growth":   10_000_000,
}

// PlanPrices maps plan names to Stripe price IDs.
var PlanPrices = map[string]string{
	"dev":      "price_dev_monthly",       // set via STRIPE_PRICE_* env vars
	"startup":  "price_startup_monthly",
	"growth":   "price_growth_monthly",
}

type StripeService struct {
	secretKey          string
	webhookSecret      string
	subscriptionCache map[string]string // subID -> workspaceID (in-memory for MVP)
}

func NewStripeService(secretKey, webhookSecret string) *StripeService {
	stripe.Key = secretKey
	return &StripeService{
		secretKey:          secretKey,
		webhookSecret:      webhookSecret,
		subscriptionCache: make(map[string]string),
	}
}

// HandleWebhook processes incoming Stripe webhook events.
// It verifies the signature and dispatches to the appropriate handler.
func (s *StripeService) HandleWebhook(ctx context.Context, payload []byte, sig string) error {
	if s.webhookSecret == "" {
		log.Println("[billing] WARNING: STRIPE_WEBHOOK_SECRET not set, skipping verification")
	} else {
		event, err := webhook.ConstructEvent(payload, sig, s.webhookSecret)
		if err != nil {
			log.Printf("[billing] Invalid webhook signature: %v", err)
			return ErrInvalidSignature
		}
		payload = event.RawData // use verified payload
	}

	var event struct {
		Type string `json:"type"`
		Data struct {
			Object json.RawMessage `json:"object"`
		} `json:"data"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	switch event.Type {
	case "customer.subscription.created",
		"customer.subscription.updated":
		return s.handleSubscriptionUpdate(ctx, event.Data.Object)
	case "customer.subscription.deleted":
		return s.handleSubscriptionDeleted(ctx, event.Data.Object)
	case "customer.subscription.trial_will_end":
		return s.handleTrialEnding(ctx, event.Data.Object)
	case "invoice.payment_failed":
		return s.handlePaymentFailed(ctx, event.Data.Object)
	default:
		log.Printf("[billing] Unhandled event type: %s", event.Type)
	}

	return nil
}

type subscriptionObject struct {
	ID               string    `json:"id"`
	Status           string    `json:"status"`
	CustomerID       string    `json:"customer"`
	Metadata         struct {
		WorkspaceID string `json:"workspace_id"`
	} `json:"metadata"`
	CurrentPeriodEnd int64 `json:"current_period_end"`
}

func (s *StripeService) handleSubscriptionUpdate(ctx context.Context, raw json.RawMessage) error {
	var sub subscriptionObject
	if err := json.Unmarshal(raw, &sub); err != nil {
		return err
	}

	if sub.Metadata.WorkspaceID == "" {
		log.Println("[billing] Subscription missing workspace_id metadata")
		return nil
	}

	s.subscriptionCache[sub.ID] = sub.Metadata.WorkspaceID
	log.Printf("[billing] Subscription %s updated for workspace %s (status: %s)",
		sub.ID, sub.Metadata.WorkspaceID, sub.Status)

	// TODO: Persist to database — update workspace plan, subscription ID, period end
	_ = ctx // suppress unused warning; real impl calls storage.UpdateWorkspaceSubscription

	return nil
}

func (s *StripeService) handleSubscriptionDeleted(ctx context.Context, raw json.RawMessage) error {
	var sub subscriptionObject
	if err := json.Unmarshal(raw, &sub); err != nil {
		return err
	}

	delete(s.subscriptionCache, sub.ID)
	log.Printf("[billing] Subscription %s deleted", sub.ID)

	// TODO: Downgrade workspace to free plan
	return nil
}

func (s *StripeService) handleTrialEnding(ctx context.Context, raw json.RawMessage) error {
	var sub subscriptionObject
	if err := json.Unmarshal(raw, &sub); err != nil {
		return err
	}
	log.Printf("[billing] Trial ending for subscription %s at %d", sub.ID, sub.CurrentPeriodEnd)
	// TODO: Send email notification
	return nil
}

func (s *StripeService) handlePaymentFailed(ctx context.Context, raw json.RawMessage) error {
	var invoice struct {
		SubscriptionID string `json:"subscription"`
		CustomerID      string `json:"customer"`
		SubTotalCents  int    `json:"subtotal"`
	}
	if err := json.Unmarshal(raw, &invoice); err != nil {
		return err
	}
	log.Printf("[billing] Payment failed for subscription %s", invoice.SubscriptionID)
	// TODO: Notify workspace, grace period before downgrade
	return nil
}

// GetPlanLimit returns the monthly trace limit for a given plan.
func GetPlanLimit(plan string) int64 {
	if limit, ok := PlanLimits[plan]; ok {
		return limit
	}
	return PlanLimits["free"]
}

// GetPlanFromPriceID resolves a Stripe price ID to a plan name.
// In production this is stored in a database. For MVP we use env vars.
func GetPlanFromPriceID(priceID string) string {
	for plan, p := range PlanPrices {
		if p == priceID {
			return plan
		}
	}
	return "free"
}

// CreateCheckoutSession is a placeholder for Stripe Checkout integration.
// Usage: redirect user to checkout URL, then Stripe calls webhook on success.
func (s *StripeService) CreateCheckoutSession(workspaceID, priceID, successURL, cancelURL string) (string, error) {
	if s.secretKey == "" {
		return "", errors.New("stripe not configured")
	}
	// TODO: Implement Stripe Checkout Session creation
	_ = workspaceID
	_ = priceID
	_ = successURL
	_ = cancelURL
	return "https://checkout.stripe.com/placeholder", nil
}

// RecordUsage records a trace event for usage-based billing metering.
// Uses Stripe's metering API for usage-based tiers.
func (s *StripeService) RecordUsage(subscriptionID string, quantity int64) error {
	if s.secretKey == "" || subscriptionID == "" {
		return nil // no-op in dev mode
	}

	params := &stripe.BillingMeterEventParams{}
	params.SetSubscription(subscriptionID)
	params.SetQuantity(quantity)
	params.SetTimestamp(time.Now().Unix())

	_, err := billingmeter.NewEvent(params)
	return err
}

// GetSubscription retrieves subscription details for a workspace.
func (s *StripeService) GetSubscription(ctx context.Context, subscriptionID string) (*subscriptionObject, error) {
	if subID, ok := s.subscriptionCache[subscriptionID]; ok && subID != "" {
		// In real impl: fetch from Stripe API or DB
		return &subscriptionObject{ID: subscriptionID, Status: "active"}, nil
	}
	return nil, ErrSubscriptionNotFound
}
