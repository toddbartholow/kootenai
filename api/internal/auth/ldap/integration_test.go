//go:build integration

package ldap

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

// Run with: go test -tags integration -run TestLive ./internal/auth/ldap/...
//
// Requires a running FreeIPA server at <FREEIPA_IP> with test users created
// by the setup script (testadmin, testinstructor, teststudent).

func liveConfig() Config {
	return Config{
		Host:             "<FREEIPA_IP>",
		Port:             636,
		UseTLS:           true,
		TLSSkipVerify:    true, // cert is issued for hostname, not IP
		BindDN:           "uid=admin,cn=users,cn=accounts,dc=example,dc=com",
		BindPassword:     "<EXAMPLE_PASSWORD>",
		BaseDN:           "cn=users,cn=accounts,dc=example,dc=com",
		UserSearchFilter: "(uid=%s)",
		UserAttrMap:      DefaultUserAttributeMap(),
		GroupRoleMapping: map[string]string{
			"cn=lab-admins,cn=groups,cn=accounts,dc=example,dc=com":      "admin",
			"cn=lab-instructors,cn=groups,cn=accounts,dc=example,dc=com": "instructor",
			"cn=lab-students,cn=groups,cn=accounts,dc=example,dc=com":    "student",
		},
		DefaultRole:     "student",
		MaxIdleConns:    2,
		ConnIdleTimeout: 30 * time.Second,
		SearchTimeout:   10 * time.Second,
	}
}

func TestLive_HealthCheck(t *testing.T) {
	client, err := NewClient(liveConfig(), slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err := client.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
	t.Log("HealthCheck passed")
}

func TestLive_Authenticate_Student(t *testing.T) {
	client, err := NewClient(liveConfig(), slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	info, err := client.Authenticate(context.Background(), "teststudent", "<EXAMPLE_PASSWORD>")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	t.Logf("User: uid=%s, email=%s, name=%s, role=%s, groups=%v", info.UID, info.Email, info.DisplayName, info.Role, info.Groups)

	if info.UID != "teststudent" {
		t.Errorf("UID = %q, want teststudent", info.UID)
	}
	if info.Email != "teststudent@lab.local" {
		t.Errorf("Email = %q, want teststudent@lab.local", info.Email)
	}
	if info.DisplayName != "Test Student" {
		t.Errorf("DisplayName = %q, want Test Student", info.DisplayName)
	}
	if info.Role != "student" {
		t.Errorf("Role = %q, want student", info.Role)
	}
}

func TestLive_Authenticate_Admin(t *testing.T) {
	client, err := NewClient(liveConfig(), slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	info, err := client.Authenticate(context.Background(), "testadmin", "<EXAMPLE_PASSWORD>")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	t.Logf("User: uid=%s, role=%s, groups=%v", info.UID, info.Role, info.Groups)

	if info.Role != "admin" {
		t.Errorf("Role = %q, want admin (from lab-admins group)", info.Role)
	}
}

func TestLive_Authenticate_Instructor(t *testing.T) {
	client, err := NewClient(liveConfig(), slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	info, err := client.Authenticate(context.Background(), "testinstructor", "<EXAMPLE_PASSWORD>")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	t.Logf("User: uid=%s, role=%s, groups=%v", info.UID, info.Role, info.Groups)

	if info.Role != "instructor" {
		t.Errorf("Role = %q, want instructor (from lab-instructors group)", info.Role)
	}
}

func TestLive_Authenticate_WrongPassword(t *testing.T) {
	client, err := NewClient(liveConfig(), slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	_, err = client.Authenticate(context.Background(), "teststudent", "wrongpassword")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if err != ErrInvalidCredentials {
		t.Errorf("error = %v, want ErrInvalidCredentials", err)
	}
}

func TestLive_Authenticate_UserNotFound(t *testing.T) {
	client, err := NewClient(liveConfig(), slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	_, err = client.Authenticate(context.Background(), "nonexistent", "password")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
	if err != ErrUserNotFound {
		t.Errorf("error = %v, want ErrUserNotFound", err)
	}
}
