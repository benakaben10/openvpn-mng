package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tldr-it-stepankutaj/openvpn-mng/internal/middleware"
	"github.com/tldr-it-stepankutaj/openvpn-mng/internal/models"
	"github.com/tldr-it-stepankutaj/openvpn-mng/test/testutil"
)

// This test lives under test/ rather than beside the package: testutil pulls in
// internal/services, which imports internal/middleware, so an in-package test
// file would create an import cycle.
func TestAuditLoggerRetentionLimit(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer testutil.CleanupTestDB(t, db)

	middleware.SetAuditLogMaxEntries(2)
	defer middleware.SetAuditLogMaxEntries(middleware.DefaultAuditLogMaxEntries)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	logger := middleware.NewAuditLogger()
	userID := uuid.New()

	for i := 0; i < 3; i++ {
		require.NoError(t, logger.LogLogin(c, userID, "test login"))
	}

	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).Count(&count).Error)
	assert.Equal(t, int64(2), count)
}
