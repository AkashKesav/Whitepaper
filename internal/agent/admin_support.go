package agent

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// Ticket Structures (matching Schema)
type TicketDTO struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	Priority   string    `json:"priority"`
	CreatedBy  string    `json:"created_by"`
	AssignedTo string    `json:"assigned_to"`
	CreatedAt  time.Time `json:"created_at"`
}

// handleListTickets returns all support tickets
// GET /api/admin/support/tickets
func (s *Server) handleListTickets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Fetch Tickets from Redis
	val, err := s.agent.RedisClient.HGetAll(ctx, "support:tickets").Result()
	if err != nil {
		s.logger.Error("Failed to fetch tickets from Redis", zap.Error(err))
		http.Error(w, "Failed to fetch tickets", http.StatusInternalServerError)
		return
	}

	tickets := []TicketDTO{}
	for _, jsonTicket := range val {
		var t TicketDTO
		if err := json.Unmarshal([]byte(jsonTicket), &t); err == nil {
			tickets = append(tickets, t)
		}
	}

	// 2. If no tickets, seed some defaults for MVP (so the UI isn't empty)
	if len(tickets) == 0 {
		defaultTickets := []TicketDTO{
			{ID: "tkt_1", Title: "Cannot access workspace", Status: "open", Priority: "high", CreatedBy: "alice", CreatedAt: time.Now().Add(-2 * time.Hour)},
			{ID: "tkt_2", Title: "Billing inquiry", Status: "closed", Priority: "low", CreatedBy: "bob", CreatedAt: time.Now().Add(-24 * time.Hour)},
		}
		for _, t := range defaultTickets {
			data, _ := json.Marshal(t)
			s.agent.RedisClient.HSet(ctx, "support:tickets", t.ID, data)
			tickets = append(tickets, t)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tickets": tickets,
	})
}

// handleResolveTicket closes a ticket
// POST /api/admin/support/tickets/{id}/resolve
func (s *Server) handleResolveTicket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ticketID := vars["id"]
	ctx := r.Context()

	// 1. Get existing ticket
	val, err := s.agent.RedisClient.HGet(ctx, "support:tickets", ticketID).Result()
	if err != nil {
		http.Error(w, "Ticket not found", http.StatusNotFound)
		return
	}

	// 2. Update Status
	var t TicketDTO
	if err := json.Unmarshal([]byte(val), &t); err != nil {
		http.Error(w, "Data corruption", http.StatusInternalServerError)
		return
	}
	t.Status = "closed"

	// 3. Save back to Redis
	newData, _ := json.Marshal(t)
	if err := s.agent.RedisClient.HSet(ctx, "support:tickets", ticketID, newData).Err(); err != nil {
		http.Error(w, "Failed to update ticket", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Ticket resolved", zap.String("ticket_id", ticketID))

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "resolved"})
}
