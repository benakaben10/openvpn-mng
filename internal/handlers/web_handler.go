package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tldr-it-stepankutaj/openvpn-mng/internal/middleware"
	"github.com/tldr-it-stepankutaj/openvpn-mng/internal/models"
	"github.com/tldr-it-stepankutaj/openvpn-mng/internal/services"
)

const webListPageSize = 20

func webListPage(c *gin.Context) int {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func webPagination(total int64, page int) gin.H {
	totalPages := services.CalculateTotalPages(total, webListPageSize)
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	prevPage := page - 1
	if prevPage < 1 {
		prevPage = 1
	}
	nextPage := page + 1
	if totalPages < 1 || nextPage > totalPages {
		nextPage = page
	}
	return gin.H{
		"page":       page,
		"pageSize":   webListPageSize,
		"total":      total,
		"totalPages": totalPages,
		"hasPrev":    page > 1,
		"hasNext":    totalPages > 0 && page < totalPages,
		"prevPage":   prevPage,
		"nextPage":   nextPage,
	}
}

// WebHandler handles web page requests
type WebHandler struct {
	userService      *services.UserService
	groupService     *services.GroupService
	networkService   *services.NetworkService
	dashboardService *services.DashboardService
}

// NewWebHandler creates a new web handler
func NewWebHandler() *WebHandler {
	return &WebHandler{
		userService:      services.NewUserService(),
		groupService:     services.NewGroupService(),
		networkService:   services.NewNetworkService(),
		dashboardService: services.NewDashboardService(),
	}
}

// pageData builds the payload every authenticated page shares with the layout
// partials: the signed-in account for the user menu, the role that drives
// navigation visibility, and the key of the active navigation entry.
//
// active must always be set — the sidebar compares it with string literals and
// a missing key would make the template comparison fail at render time.
func (h *WebHandler) pageData(c *gin.Context, title, active string) gin.H {
	authUser := middleware.GetAuthUser(c)
	authUserID := middleware.GetAuthUserID(c)

	// A nil user is tolerated by the layout; it only degrades the user menu.
	user, _ := h.userService.GetByID(authUserID)

	return gin.H{
		"title":  title,
		"role":   authUser.Role,
		"user":   user,
		"active": active,
	}
}

// renderError renders the standalone error page.
func (h *WebHandler) renderError(c *gin.Context, status int, title, message string) {
	c.HTML(status, "error.html", gin.H{
		"title":   title + " - OpenVPN Manager",
		"heading": title,
		"message": message,
		"status":  status,
	})
}

// requireAdmin renders the access-denied page and reports whether the caller
// may continue.
func (h *WebHandler) requireAdmin(c *gin.Context) bool {
	if middleware.GetAuthUser(c).Role == models.RoleAdmin {
		return true
	}
	h.renderError(c, http.StatusForbidden, "Access denied",
		"This area is restricted to administrators.")
	return false
}

// IndexPage redirects to login
func (h *WebHandler) IndexPage(c *gin.Context) {
	c.Redirect(http.StatusFound, "/login")
}

// LoginPage renders the login page
func (h *WebHandler) LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Sign in - OpenVPN Manager",
	})
}

// DashboardPage renders the dashboard page
func (h *WebHandler) DashboardPage(c *gin.Context) {
	authUser := middleware.GetAuthUser(c)
	authUserID := middleware.GetAuthUserID(c)

	data := h.pageData(c, "Dashboard - OpenVPN Manager", "dashboard")

	// Add stats for ADMIN role
	if authUser.Role == models.RoleAdmin {
		stats, err := h.dashboardService.GetAdminStats()
		if err == nil {
			data["stats"] = stats
		}
	}

	// Add subordinate count for MANAGER role
	if authUser.Role == models.RoleManager {
		count, err := h.dashboardService.GetManagerSubordinateCount(authUserID.String())
		if err == nil {
			data["subordinateCount"] = count
		}
	}

	c.HTML(http.StatusOK, "dashboard.html", data)
}

// UsersPage renders the users list page
func (h *WebHandler) UsersPage(c *gin.Context) {
	authUser := middleware.GetAuthUser(c)
	authUserID := middleware.GetAuthUserID(c)
	page := webListPage(c)

	users, total, err := h.userService.List(page, webListPageSize, authUser.Role, &authUserID)
	if err != nil {
		h.renderError(c, http.StatusInternalServerError, "Something went wrong",
			"Failed to load users.")
		return
	}

	data := h.pageData(c, "Users - OpenVPN Manager", "users")
	data["users"] = users
	for key, value := range webPagination(total, page) {
		data[key] = value
	}
	c.HTML(http.StatusOK, "users.html", data)
}

// UserDetailPage renders the user detail page
func (h *WebHandler) UserDetailPage(c *gin.Context) {
	data := h.pageData(c, "User detail - OpenVPN Manager", "users")
	data["userID"] = c.Param("id")
	c.HTML(http.StatusOK, "user_detail.html", data)
}

// GroupsPage renders the groups list page
func (h *WebHandler) GroupsPage(c *gin.Context) {
	page := webListPage(c)

	groups, total, err := h.groupService.List(page, webListPageSize)
	if err != nil {
		h.renderError(c, http.StatusInternalServerError, "Something went wrong",
			"Failed to load groups.")
		return
	}

	data := h.pageData(c, "Groups - OpenVPN Manager", "groups")
	data["groups"] = groups
	for key, value := range webPagination(total, page) {
		data[key] = value
	}
	c.HTML(http.StatusOK, "groups.html", data)
}

// ProfilePage renders the profile page
func (h *WebHandler) ProfilePage(c *gin.Context) {
	data := h.pageData(c, "My profile - OpenVPN Manager", "profile")
	if data["user"] == nil {
		h.renderError(c, http.StatusInternalServerError, "Something went wrong",
			"Failed to load your profile.")
		return
	}
	c.HTML(http.StatusOK, "profile.html", data)
}

// NetworksPage renders the networks list page (ADMIN only)
func (h *WebHandler) NetworksPage(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}

	page := webListPage(c)
	networks, total, err := h.networkService.List(page, webListPageSize)
	if err != nil {
		h.renderError(c, http.StatusInternalServerError, "Something went wrong",
			"Failed to load networks.")
		return
	}

	data := h.pageData(c, "Networks - OpenVPN Manager", "networks")
	data["networks"] = networks
	for key, value := range webPagination(total, page) {
		data[key] = value
	}
	c.HTML(http.StatusOK, "networks.html", data)
}

// AuditPage renders the audit logs page (ADMIN only)
func (h *WebHandler) AuditPage(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	c.HTML(http.StatusOK, "audit.html", h.pageData(c, "Audit log - OpenVPN Manager", "audit"))
}

// SessionsPage renders the VPN sessions history page (ADMIN only)
func (h *WebHandler) SessionsPage(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	c.HTML(http.StatusOK, "sessions.html", h.pageData(c, "Session history - OpenVPN Manager", "sessions"))
}
