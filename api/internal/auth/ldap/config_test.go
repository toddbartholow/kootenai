package ldap

import "testing"

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "missing host",
			cfg:     Config{UseTLS: true, BindDN: "cn=svc", BindPassword: "pw", BaseDN: "dc=example"},
			wantErr: "host is required",
		},
		{
			name:    "no TLS",
			cfg:     Config{Host: "ldap.example.com", BindDN: "cn=svc", BindPassword: "pw", BaseDN: "dc=example"},
			wantErr: "TLS or StartTLS is required",
		},
		{
			name:    "missing bind DN",
			cfg:     Config{Host: "ldap.example.com", UseTLS: true, BindPassword: "pw", BaseDN: "dc=example"},
			wantErr: "bind_dn",
		},
		{
			name:    "missing bind password",
			cfg:     Config{Host: "ldap.example.com", UseTLS: true, BindDN: "cn=svc", BaseDN: "dc=example"},
			wantErr: "bind_password",
		},
		{
			name:    "missing base DN",
			cfg:     Config{Host: "ldap.example.com", UseTLS: true, BindDN: "cn=svc", BindPassword: "pw"},
			wantErr: "base_dn",
		},
		{
			name: "valid config",
			cfg:  Config{Host: "ldap.example.com", UseTLS: true, BindDN: "cn=svc", BindPassword: "pw", BaseDN: "dc=example"},
		},
		{
			name: "valid with StartTLS",
			cfg:  Config{Host: "ldap.example.com", StartTLS: true, BindDN: "cn=svc", BindPassword: "pw", BaseDN: "dc=example"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestConfig_Addr(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "explicit port", cfg: Config{Host: "ldap.example.com", Port: 636}, want: "ldap.example.com:636"},
		{name: "default LDAPS", cfg: Config{Host: "ldap.example.com", UseTLS: true}, want: "ldap.example.com:636"},
		{name: "default StartTLS", cfg: Config{Host: "ldap.example.com", StartTLS: true}, want: "ldap.example.com:389"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.Addr()
			if got != tt.want {
				t.Errorf("Addr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfig_MapGroupsToRoles(t *testing.T) {
	cfg := Config{
		GroupRoleMapping: map[string]string{
			"cn=admins,dc=example":      "admin",
			"cn=instructors,dc=example": "instructor",
			"cn=students,dc=example":    "student",
		},
		DefaultRole: "student",
	}

	tests := []struct {
		name   string
		groups []string
		want   string
	}{
		{name: "admin group", groups: []string{"cn=admins,dc=example"}, want: "admin"},
		{name: "instructor group", groups: []string{"cn=instructors,dc=example"}, want: "instructor"},
		{name: "multiple groups — highest wins", groups: []string{"cn=students,dc=example", "cn=admins,dc=example"}, want: "admin"},
		{name: "no matching group", groups: []string{"cn=unknown,dc=example"}, want: "student"},
		{name: "empty groups", groups: nil, want: "student"},
		{name: "case insensitive", groups: []string{"CN=Admins,DC=Example"}, want: "admin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.MapGroupsToRoles(tt.groups)
			if got != tt.want {
				t.Errorf("MapGroupsToRoles() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfig_SearchFilter(t *testing.T) {
	cfg := Config{UserSearchFilter: "(uid=%s)"}
	got := cfg.SearchFilter("john")
	if got != "(uid=john)" {
		t.Errorf("SearchFilter() = %q, want %q", got, "(uid=john)")
	}

	// Test LDAP injection prevention
	got = cfg.SearchFilter("john*()")
	if got != "(uid=john\\2a\\28\\29)" {
		t.Errorf("SearchFilter() should escape specials, got %q", got)
	}
}

func TestLdapEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal", "normal"},
		{"user*", "user\\2a"},
		{"user()", "user\\28\\29"},
		{"user\\name", "user\\5cname"},
		{"user\x00", "user\\00"},
	}
	for _, tt := range tests {
		got := ldapEscape(tt.input)
		if got != tt.want {
			t.Errorf("ldapEscape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
