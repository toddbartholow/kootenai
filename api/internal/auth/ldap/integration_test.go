//go:build integration

package ldap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Live tests against a real FreeIPA directory:
//
//	go test -tags integration -run TestLive ./internal/auth/ldap/...
//
// The DNs below assume FreeIPA's container layout -- cn=users,cn=accounts,<suffix>
// and cn=groups,cn=accounts,<suffix> -- which is not overridable here, so these
// tests do not transfer to OpenLDAP, 389-DS or AD without editing liveConfig.
//
// Every setting comes from the environment and each test skips when a required
// variable is unset, so this file carries no server address and no credentials:
//
//	KOOTENAI_LDAP_HOST             required  directory hostname, e.g. ipa.example.com
//	KOOTENAI_LDAP_BASE_DN          required  the directory suffix ONLY, e.g. dc=example,dc=com.
//	                                         NOT the application's LDAP_BASE_DN, which is the
//	                                         full user-search base; the containers are added below.
//	KOOTENAI_LDAP_BIND_PASSWORD    required  password for the bind account
//	KOOTENAI_LDAP_TEST_PASSWORD    required  by the three authentication tests only: the
//	                                         shared password of the three fixture users
//	KOOTENAI_LDAP_BIND_UID         optional  bind account uid (default "admin")
//	KOOTENAI_LDAP_PORT             optional  default 636. LDAPS only -- StartTLS and custom
//	                                         CA certificates are not exposed here.
//	KOOTENAI_LDAP_MAIL_DOMAIN      optional  default: the base DN's dc components joined by
//	                                         ".". Required when the suffix has no dc= parts.
//	KOOTENAI_LDAP_TLS_SKIP_VERIFY  optional  any strconv.ParseBool value; set it when
//	                                         connecting by IP, since FreeIPA issues its
//	                                         certificate for the hostname. Note the
//	                                         application's LDAP_TLS_SKIP_VERIFY accepts only
//	                                         "true", so "1" means the opposite there.
//
// The tests expect three users -- teststudent, testinstructor and testadmin --
// in the groups lab-students, lab-instructors and lab-admins respectively, all
// sharing KOOTENAI_LDAP_TEST_PASSWORD. teststudent additionally needs mail
// teststudent@<mail domain> and displayName (or givenName plus sn) "Test
// Student", both of which TestLive_Authenticate_Student asserts on. Creating
// them is left to the operator; this repository ships no script for it.

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("%s is not set; skipping live LDAP test", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// mailDomainFrom turns "dc=example,dc=com" into "example.com".
func mailDomainFrom(baseDN string) string {
	var labels []string
	for _, rdn := range strings.Split(baseDN, ",") {
		rdn = strings.TrimSpace(rdn)
		if len(rdn) > 3 && strings.EqualFold(rdn[:3], "dc=") {
			labels = append(labels, rdn[3:])
		}
	}
	return strings.Join(labels, ".")
}

// liveMailDomain resolves the domain the fixture users' mail attribute should
// end with. It demands KOOTENAI_LDAP_BASE_DN itself rather than relying on
// liveConfig having already required it, so the derivation cannot silently
// yield "" if that ordering ever changes.
func liveMailDomain(t *testing.T) string {
	t.Helper()

	if v := os.Getenv("KOOTENAI_LDAP_MAIL_DOMAIN"); v != "" {
		return v
	}
	domain := mailDomainFrom(requireEnv(t, "KOOTENAI_LDAP_BASE_DN"))
	if domain == "" {
		t.Skip("set KOOTENAI_LDAP_MAIL_DOMAIN: the base DN has no dc= components to derive it from")
	}
	return domain
}

func liveConfig(t *testing.T) Config {
	t.Helper()

	host := requireEnv(t, "KOOTENAI_LDAP_HOST")
	baseDN := requireEnv(t, "KOOTENAI_LDAP_BASE_DN")
	bindPassword := requireEnv(t, "KOOTENAI_LDAP_BIND_PASSWORD")

	port := 636
	if raw := os.Getenv("KOOTENAI_LDAP_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("KOOTENAI_LDAP_PORT = %q: %v", raw, err)
		}
		port = parsed
	}

	skipVerify := false
	if raw := os.Getenv("KOOTENAI_LDAP_TLS_SKIP_VERIFY"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			t.Fatalf("KOOTENAI_LDAP_TLS_SKIP_VERIFY = %q: %v", raw, err)
		}
		skipVerify = parsed
	}

	return Config{
		Host:             host,
		Port:             port,
		UseTLS:           true,
		TLSSkipVerify:    skipVerify,
		BindDN:           fmt.Sprintf("uid=%s,cn=users,cn=accounts,%s", envOr("KOOTENAI_LDAP_BIND_UID", "admin"), baseDN),
		BindPassword:     bindPassword,
		BaseDN:           fmt.Sprintf("cn=users,cn=accounts,%s", baseDN),
		UserSearchFilter: "(uid=%s)",
		UserAttrMap:      DefaultUserAttributeMap(),
		GroupRoleMapping: map[string]string{
			fmt.Sprintf("cn=lab-admins,cn=groups,cn=accounts,%s", baseDN):      "admin",
			fmt.Sprintf("cn=lab-instructors,cn=groups,cn=accounts,%s", baseDN): "instructor",
			fmt.Sprintf("cn=lab-students,cn=groups,cn=accounts,%s", baseDN):    "student",
		},
		DefaultRole:     "student",
		MaxIdleConns:    2,
		ConnIdleTimeout: 30 * time.Second,
		SearchTimeout:   10 * time.Second,
	}
}

func newLiveClient(t *testing.T) *Client {
	t.Helper()

	client, err := NewClient(liveConfig(t), slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestLive_HealthCheck(t *testing.T) {
	client := newLiveClient(t)

	if err := client.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
	t.Log("HealthCheck passed")
}

func TestLive_Authenticate_Student(t *testing.T) {
	client := newLiveClient(t)
	password := requireEnv(t, "KOOTENAI_LDAP_TEST_PASSWORD")
	mailDomain := liveMailDomain(t)

	info, err := client.Authenticate(context.Background(), "teststudent", password)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	t.Logf("User: uid=%s, email=%s, name=%s, role=%s, groups=%v", info.UID, info.Email, info.DisplayName, info.Role, info.Groups)

	if info.UID != "teststudent" {
		t.Errorf("UID = %q, want teststudent", info.UID)
	}
	if want := "teststudent@" + mailDomain; !strings.EqualFold(info.Email, want) {
		t.Errorf("Email = %q, want %q", info.Email, want)
	}
	if info.DisplayName != "Test Student" {
		t.Errorf("DisplayName = %q, want Test Student", info.DisplayName)
	}
	if info.Role != "student" {
		t.Errorf("Role = %q, want student", info.Role)
	}
}

func TestLive_Authenticate_Admin(t *testing.T) {
	client := newLiveClient(t)
	password := requireEnv(t, "KOOTENAI_LDAP_TEST_PASSWORD")

	info, err := client.Authenticate(context.Background(), "testadmin", password)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	t.Logf("User: uid=%s, role=%s, groups=%v", info.UID, info.Role, info.Groups)

	if info.Role != "admin" {
		t.Errorf("Role = %q, want admin (from lab-admins group)", info.Role)
	}
}

func TestLive_Authenticate_Instructor(t *testing.T) {
	client := newLiveClient(t)
	password := requireEnv(t, "KOOTENAI_LDAP_TEST_PASSWORD")

	info, err := client.Authenticate(context.Background(), "testinstructor", password)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	t.Logf("User: uid=%s, role=%s, groups=%v", info.UID, info.Role, info.Groups)

	if info.Role != "instructor" {
		t.Errorf("Role = %q, want instructor (from lab-instructors group)", info.Role)
	}
}

func TestLive_Authenticate_WrongPassword(t *testing.T) {
	client := newLiveClient(t)

	_, err := client.Authenticate(context.Background(), "teststudent", "definitely-not-the-password")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("error = %v, want ErrInvalidCredentials", err)
	}
}

func TestLive_Authenticate_UserNotFound(t *testing.T) {
	client := newLiveClient(t)

	_, err := client.Authenticate(context.Background(), "nonexistent", "definitely-not-the-password")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("error = %v, want ErrUserNotFound", err)
	}
}
