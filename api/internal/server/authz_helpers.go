// Authz helpers are defined in serverutil and re-exported here as aliases for
// backward compatibility. New sub-packages should import serverutil directly.
package server

import (
	"net/http"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/server/serverutil"
)

func isAdminOrInstructor(user *auth.User) bool { return serverutil.IsAdminOrInstructor(user) }
func isAdminRequest(r *http.Request) bool      { return serverutil.IsAdminRequest(r) }
