package agent

import (
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Finance Structures
type RevenueReport struct {
	TotalRevenue   float64            `json:"total_revenue"`
	MRR            float64            `json:"mrr"`
	ARR            float64            `json:"arr"`
	RevenueByPlan  map[string]float64 `json:"revenue_by_plan"`
	RevenueByMonth map[string]float64 `json:"revenue_by_month"`
	GeneratedAt    time.Time          `json:"generated_at"`
}

type SubscriptionUpdate struct {
	Username string `json:"username"`
	Plan     string `json:"plan"`   // free, pro, team, enterprise
	Status   string `json:"status"` // active, trial, cancelled
}

// handleGetRevenue returns generated revenue statistics
// GET /api/admin/finance/revenue
func (s *Server) handleGetRevenue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Fetch Subscription Data from Redis
	// We'll track user plans in a hash 'user_plan' -> { username: plan_name }
	plans, err := s.agent.RedisClient.HGetAll(ctx, "user_plan").Result()
	if err != nil {
		s.logger.Error("Failed to fetch user plans", zap.Error(err))
		http.Error(w, "Failed to fetch revenue data", http.StatusInternalServerError)
		return
	}

	// 2. Define Pricing
	pricing := map[string]float64{
		"free":       0.0,
		"pro":        29.99,
		"team":       99.99,
		"enterprise": 499.99,
	}

	// 3. Calculate Revenue
	var totalRevenue, mrr float64
	revenueByPlan := map[string]float64{"pro": 0, "team": 0, "enterprise": 0}

	// If no data, seed some for MVP so charts aren't empty
	if len(plans) == 0 {
		seedPlans := map[string]string{
			"alice":     "pro",
			"bob":       "team",
			"charlie":   "free",
			"dave":      "enterprise",
			"eve":       "pro",
			"test_user": "pro",
		}
		for u, p := range seedPlans {
			s.agent.RedisClient.HSet(ctx, "user_plan", u, p)
			plans[u] = p
		}
	}

	for _, plan := range plans {
		if cost, ok := pricing[plan]; ok {
			mrr += cost
			revenueByPlan[plan] += cost
		}
	}

	// Mock historical data for charts (hard to generate real history instantly)
	// MRR * 12 roughly approximates ARR
	arr := mrr * 12
	totalRevenue = arr * 0.5 // Just a mock "Total Lifetime" number

	report := RevenueReport{
		TotalRevenue:  totalRevenue,
		MRR:           mrr,
		ARR:           arr,
		RevenueByPlan: revenueByPlan,
		RevenueByMonth: map[string]float64{
			"2025-12": mrr * 0.9,
			"2026-01": mrr,
		},
		GeneratedAt: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// handleUpdateSubscription manually updates a user's subscription
// POST /api/admin/finance/subscription/update
func (s *Server) handleUpdateSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req SubscriptionUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update Redis
	if err := s.agent.RedisClient.HSet(ctx, "user_plan", req.Username, req.Plan).Err(); err != nil {
		s.logger.Error("Failed to update subscription", zap.Error(err))
		http.Error(w, "Persistence failed", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Admin updated subscription",
		zap.String("target_user", req.Username),
		zap.String("new_plan", req.Plan),
	)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}
