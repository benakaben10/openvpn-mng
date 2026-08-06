package templates_test

import (
	"html/template"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tldr-it-stepankutaj/openvpn-mng/internal/models"
	"github.com/tldr-it-stepankutaj/openvpn-mng/internal/services"
)

// templateGlob mirrors the pattern routes.SetupRoutes passes to LoadHTMLGlob.
const templateGlob = "web/templates/*"

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)

	// Walk up until go.mod is found so the test works from any package dir.
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		if filepath.Dir(dir) == dir {
			t.Fatalf("go.mod not found above %s", wd)
		}
	}
}

func parseTemplates(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := template.ParseGlob(filepath.Join(repoRoot(t), templateGlob))
	require.NoError(t, err, "templates must parse the same way Gin loads them")
	return tmpl
}

func sampleUser(role models.Role) *models.User {
	updated := time.Date(2026, 2, 1, 9, 30, 0, 0, time.UTC)
	validFrom := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	validTo := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	managerID := uuid.New()

	return &models.User{
		ID:         uuid.New(),
		Username:   "jdoe",
		FirstName:  "Jane",
		MiddleName: "Q",
		LastName:   "Doe",
		Email:      "jane@example.com",
		Telephone:  "+420123456789",
		Role:       role,
		IsActive:   true,
		ValidFrom:  &validFrom,
		ValidTo:    &validTo,
		VpnIP:      "10.8.0.12",
		CreatedAt:  time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC),
		UpdatedAt:  &updated,
		ManagerID:  &managerID,
		Manager: &models.User{
			FirstName: "Ada",
			LastName:  "Lovelace",
		},
	}
}

// pageData mirrors handlers.WebHandler.pageData plus the list payload the
// layout partials and page bodies read.
func pageData(role models.Role, active string, extra map[string]any) map[string]any {
	data := map[string]any{
		"title":      "Test - OpenVPN Manager",
		"role":       role,
		"user":       sampleUser(role),
		"active":     active,
		"page":       1,
		"pageSize":   20,
		"total":      int64(1),
		"totalPages": 2,
		"hasPrev":    false,
		"hasNext":    true,
		"prevPage":   1,
		"nextPage":   2,
	}
	for key, value := range extra {
		data[key] = value
	}
	return data
}

func sampleGroups() []models.Group {
	updated := time.Now()
	return []models.Group{{
		ID:          uuid.New(),
		Name:        "Engineering",
		Description: "Backend team",
		CreatedAt:   time.Now(),
		UpdatedAt:   &updated,
		CreatedBy:   uuid.New(),
	}}
}

func sampleNetworks() []models.Network {
	return []models.Network{{
		ID:          uuid.New(),
		Name:        "Corporate LAN",
		CIDR:        "192.168.1.0/24",
		Description: "Head office",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		CreatedBy:   uuid.New(),
	}}
}

func sampleStats() *services.DashboardStats {
	entityID := uuid.New()
	return &services.DashboardStats{
		TotalUsers:     12,
		ActiveUsers:    10,
		ConnectedUsers: 3,
		TotalGroups:    4,
		TotalNetworks:  5,
		TotalSessions:  128,
		TrafficStats: []services.DailyTrafficStats{
			{Date: "2026-01-01", BytesReceived: 1024, BytesSent: 2048},
		},
		RecentAuditLogs: []models.AuditLog{{
			ID:         uuid.New(),
			UserID:     uuid.New(),
			User:       sampleUser(models.RoleAdmin),
			Action:     models.AuditActionCreate,
			EntityType: "user",
			EntityID:   &entityID,
			IPAddress:  "203.0.113.7",
			CreatedAt:  time.Now(),
		}},
	}
}

// TestTemplatesParse fails loudly if any template in the glob is malformed,
// which at runtime would only surface as a panic during LoadHTMLGlob.
func TestTemplatesParse(t *testing.T) {
	tmpl := parseTemplates(t)

	for _, name := range []string{
		"login.html", "error.html", "index.html", "dashboard.html", "users.html",
		"groups.html", "networks.html", "sessions.html", "audit.html",
		"profile.html", "user_detail.html",
	} {
		require.NotNil(t, tmpl.Lookup(name), "template %s should be loaded", name)
	}

	for _, partial := range []string{"head", "sidebar", "topbar", "shell_open", "shell_close", "scripts"} {
		require.NotNil(t, tmpl.Lookup(partial), "partial %q should be defined", partial)
	}
}

// TestTemplatesRender executes every page against the payload its handler
// supplies. Missing keys (for example .active) surface here rather than as a
// 500 in production.
func TestTemplatesRender(t *testing.T) {
	users := []models.User{*sampleUser(models.RoleUser)}

	cases := []struct {
		name string
		data map[string]any
	}{
		{"login.html", map[string]any{"title": "Sign in"}},
		{"index.html", map[string]any{}},
		{"error.html", map[string]any{
			"title": "Access denied", "heading": "Access denied",
			"message": "Restricted to administrators.", "status": 403,
		}},

		{"dashboard.html", pageData(models.RoleAdmin, "dashboard", map[string]any{"stats": sampleStats()})},
		{"dashboard.html", pageData(models.RoleManager, "dashboard", map[string]any{"subordinateCount": int64(4)})},
		{"dashboard.html", pageData(models.RoleUser, "dashboard", nil)},
		// The admin dashboard tolerates stats being absent when the query fails.
		{"dashboard.html", pageData(models.RoleAdmin, "dashboard", nil)},

		{"users.html", pageData(models.RoleAdmin, "users", map[string]any{"users": users})},
		{"users.html", pageData(models.RoleManager, "users", map[string]any{"users": users})},
		{"users.html", pageData(models.RoleAdmin, "users", map[string]any{"users": []models.User{}})},

		{"groups.html", pageData(models.RoleAdmin, "groups", map[string]any{"groups": sampleGroups()})},
		{"groups.html", pageData(models.RoleManager, "groups", map[string]any{"groups": sampleGroups()})},

		{"networks.html", pageData(models.RoleAdmin, "networks", map[string]any{"networks": sampleNetworks()})},

		{"sessions.html", pageData(models.RoleAdmin, "sessions", nil)},
		{"audit.html", pageData(models.RoleAdmin, "audit", nil)},
		{"profile.html", pageData(models.RoleUser, "profile", nil)},
		{"user_detail.html", pageData(models.RoleAdmin, "users", map[string]any{"userID": uuid.New().String()})},
	}

	for _, tc := range cases {
		role, _ := tc.data["role"].(models.Role)
		t.Run(tc.name+"/"+string(role), func(t *testing.T) {
			tmpl := parseTemplates(t)
			require.NoError(t, tmpl.ExecuteTemplate(io.Discard, tc.name, tc.data))
		})
	}
}

// TestLayoutToleratesMissingUser covers the window where GetByID fails and the
// handler still renders the shell with a nil user.
func TestLayoutToleratesMissingUser(t *testing.T) {
	tmpl := parseTemplates(t)

	data := pageData(models.RoleAdmin, "sessions", nil)
	data["user"] = (*models.User)(nil)

	require.NoError(t, tmpl.ExecuteTemplate(io.Discard, "sessions.html", data))
}
