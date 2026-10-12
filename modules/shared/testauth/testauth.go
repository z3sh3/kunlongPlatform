// Package testauth provides header-driven authentication middleware for
// console-module handler tests. It is only imported from _test files and
// never linked into production builds. The header names mimic the context
// keys set by the real middleware in middleware/auth.go.
package testauth

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const (
	// HeaderUserId identifies the acting user in tests.
	HeaderUserId = "X-Test-User-Id"
	// HeaderRole overrides the acting user's role in tests.
	HeaderRole = "X-Test-Role"
)

// UserAuth authenticates the request from HeaderUserId. It mirrors the
// context keys set by middleware.UserAuth.
func UserAuth(c *gin.Context) {
	userId, _ := strconv.Atoi(c.GetHeader(HeaderUserId))
	if userId <= 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "message": "unauthorized"})
		return
	}
	c.Set("id", userId)
	role, _ := strconv.Atoi(c.GetHeader(HeaderRole))
	c.Set("role", role)
	c.Next()
}

// AdminAuth requires HeaderUserId plus an administrator role.
func AdminAuth(c *gin.Context) {
	userId, _ := strconv.Atoi(c.GetHeader(HeaderUserId))
	role, _ := strconv.Atoi(c.GetHeader(HeaderRole))
	if userId <= 0 || role < common.RoleAdminUser {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "admin only"})
		return
	}
	c.Set("id", userId)
	c.Set("role", role)
	c.Next()
}

// DoJSON performs a JSON request against the engine as the given user and
// decodes the {"success", "message", "data"} response envelope.
func DoJSON(r http.Handler, method, target string, userId, role int, body any) (*httptest.ResponseRecorder, map[string]any, error) {
	var reader io.Reader
	if body != nil {
		data, err := common.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	if userId > 0 {
		req.Header.Set(HeaderUserId, strconv.Itoa(userId))
		req.Header.Set(HeaderRole, strconv.Itoa(role))
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	resp := make(map[string]any)
	if err := common.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		return w, nil, err
	}
	return w, resp, nil
}
