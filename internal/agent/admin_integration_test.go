package agent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestAdminCRUDIntegration verifies the Admin CRUD endpoints.
// It requires a running server and dependencies.
// Set TEST_INTEGRATION=1 to run these tests.
func TestAdminCRUDIntegration(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("Skipping integration test; set TEST_INTEGRATION=1 to run")
	}

	baseURL := "http://localhost:9090/api/admin"
	client := &http.Client{Timeout: 10 * time.Second}

	// Helper to create a request with auth (assuming mock auth or simple token for now)
	// In a real scenario, we'd need to login first to get a token.
	// For this test, we assume the server might be in a dev mode or we have a valid token.
	// We'll try to login first if possible, or just expect 401 if not.
	token := loginAndGetToken(t, "http://localhost:9090/api/login")

	t.Run("User CRUD", func(t *testing.T) {
		// 1. Create User
		newUser := map[string]string{
			"username": "testuser_crud",
			"password": "password123",
			"role":     "user",
		}
		jsonData, _ := json.Marshal(newUser)
		req, _ := http.NewRequest("POST", baseURL+"/users", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// 2. Delete User
		req, _ = http.NewRequest("DELETE", baseURL+"/users/testuser_crud", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err = client.Do(req)
		if err != nil {
			t.Fatalf("Failed to delete user: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Affiliate CRUD", func(t *testing.T) {
		// 1. Create Affiliate
		newAffiliate := map[string]interface{}{
			"code":            "TEST_AFF_001",
			"user":            "test_aff_user",
			"commission_rate": 0.15,
		}
		jsonData, _ := json.Marshal(newAffiliate)
		req, _ := http.NewRequest("POST", baseURL+"/affiliates", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to create affiliate: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// 2. Delete Affiliate
		req, _ = http.NewRequest("DELETE", baseURL+"/affiliates/TEST_AFF_001", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err = client.Do(req)
		if err != nil {
			t.Fatalf("Failed to delete affiliate: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Campaign CRUD", func(t *testing.T) {
		// 1. Create Campaign
		newCampaign := map[string]string{
			"id":              "camp_test_001",
			"name":            "Test Campaign",
			"type":            "email",
			"target_audience": "all",
			"status":          "draft",
		}
		jsonData, _ := json.Marshal(newCampaign)
		req, _ := http.NewRequest("POST", baseURL+"/operations/campaigns", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to create campaign: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// 2. Delete Campaign
		req, _ = http.NewRequest("DELETE", baseURL+"/operations/campaigns/camp_test_001", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err = client.Do(req)
		if err != nil {
			t.Fatalf("Failed to delete campaign: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func loginAndGetToken(t *testing.T, url string) string {
	// Attempt to login with a known admin credential or bootstrap one.
	// For this test environment, we might need to rely on seeded data or create a user first?
	// But /register matches public endpoint.

	// Try registering a super admin first to ensure we have one
	reg := map[string]string{
		"username": "super_admin_test",
		"password": "password123",
	}
	jsonData, _ := json.Marshal(reg)
	http.Post("http://localhost:9090/api/register", "application/json", bytes.NewBuffer(jsonData))

	// Now login
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Logf("Login failed (server down?): %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Logf("Login returned status: %d", resp.StatusCode)
		return ""
	}

	var res struct {
		Token string `json:"token"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	return res.Token
}
