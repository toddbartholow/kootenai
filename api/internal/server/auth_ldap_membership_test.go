package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	authldap "github.com/toddbartholow/kootenai/api/internal/auth/ldap"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"
)

// newLDAPTestAuthManager builds an AuthManager with just the collaborators the
// LDAP provisioning path touches. findOrProvisionLDAPUser never dereferences
// ldapClient, authService or responder, so they stay nil.
func newLDAPTestAuthManager(
	userRepo *mocks.FakeUserRepository,
	orgRepo *mocks.FakeOrganizationMembershipRepository,
) *AuthManager {
	cfg := AuthManagerConfig{
		UserRepo: userRepo,
		Logger:   newTestLogger(),
	}
	// A nil *FakeOrganizationMembershipRepository must be passed as a nil
	// interface, not a typed-nil, or the nil check in ensureSystemOrgMembership
	// would not fire.
	if orgRepo != nil {
		cfg.OrgMembershipRepo = orgRepo
	}
	return NewAuthManager(cfg)
}

// acceptedSystemMembership builds a membership that GetPrimaryOrganization
// will find: primary, into the system org, and accepted.
func acceptedSystemMembership(id, userID string) *models.OrganizationMembership {
	now := time.Now()
	return &models.OrganizationMembership{
		ID:             id,
		OrganizationID: systemOrgID,
		UserID:         userID,
		Role:           models.OrgRoleMember,
		IsPrimary:      true,
		InvitedAt:      now,
		AcceptedAt:     &now,
	}
}

// TestFindOrProvisionLDAPUser_SystemOrgMembership is the regression suite for
// the self-healing membership provisioning. The bug: membership was only
// created on the "new user" branch, and a failure there was logged and
// swallowed. Because the user row was already persisted, every later login
// short-circuited on the external-ID lookup and never retried, leaving the user
// permanently without a primary org — an empty dashboard fixable only by a
// manual DB insert.
func TestFindOrProvisionLDAPUser_SystemOrgMembership(t *testing.T) {
	const (
		ldapUID    = "jdoe"
		externalID = "ldap:" + ldapUID
		email      = "jdoe@example.edu"
	)

	ldapInfo := &authldap.UserInfo{
		UID:         ldapUID,
		Email:       email,
		DisplayName: "Jane Doe",
		Role:        "student",
	}

	tests := []struct {
		name string
		// seed populates the repos before the login attempt.
		seed func(*mocks.FakeUserRepository, *mocks.FakeOrganizationMembershipRepository)
		// wantCreates is how many membership Create calls the login should make.
		wantCreates int
		// wantPrimaryOrg is whether the user holds a primary org afterwards.
		wantPrimaryOrg bool
	}{
		{
			// Branch 3: brand-new user. Membership is created alongside the user.
			name:           "new user is provisioned into the system org",
			seed:           func(*mocks.FakeUserRepository, *mocks.FakeOrganizationMembershipRepository) {},
			wantCreates:    1,
			wantPrimaryOrg: true,
		},
		{
			// Branch 1 (healthy): returning user who already has a primary org.
			// ensureSystemOrgMembership must be a read-only no-op — no duplicate
			// membership row, which the DB's unique constraint would reject.
			name: "existing user with a primary org gets no duplicate membership",
			seed: func(u *mocks.FakeUserRepository, o *mocks.FakeOrganizationMembershipRepository) {
				u.AddUser(&models.User{
					ID:         "user-existing",
					ExternalID: externalID,
					Email:      email,
					Role:       "student",
					IsActive:   true,
				})
				o.AddMembership(acceptedSystemMembership("mem-existing", "user-existing"))
			},
			wantCreates:    0,
			wantPrimaryOrg: true,
		},
		{
			// THE CORE REGRESSION. Branch 1 with a user whose membership insert
			// failed on a previous login: the user row exists and is linked to
			// LDAP, but there is no membership. Before the fix, branch 1 returned
			// early and this user was stuck forever. Now the next login repairs it.
			name: "existing user with no primary org self-heals on next login",
			seed: func(u *mocks.FakeUserRepository, _ *mocks.FakeOrganizationMembershipRepository) {
				u.AddUser(&models.User{
					ID:         "user-orphaned",
					ExternalID: externalID,
					Email:      email,
					Role:       "student",
					IsActive:   true,
				})
			},
			wantCreates:    1,
			wantPrimaryOrg: true,
		},
		{
			// Branch 2: a local user linked to LDAP by email. Before the fix this
			// path never created a membership at all.
			name: "local user linked by email is provisioned into the system org",
			seed: func(u *mocks.FakeUserRepository, _ *mocks.FakeOrganizationMembershipRepository) {
				u.AddUser(&models.User{
					ID:       "user-local",
					Email:    email,
					Role:     "student",
					IsActive: true,
				})
			},
			wantCreates:    1,
			wantPrimaryOrg: true,
		},
		{
			// A membership that was written without AcceptedAt is invisible to
			// GetPrimaryOrganization (`accepted_at IS NOT NULL`) and to the tenant
			// middleware, so it must not be mistaken for a healthy membership.
			name: "membership with no AcceptedAt does not count as a primary org",
			seed: func(u *mocks.FakeUserRepository, o *mocks.FakeOrganizationMembershipRepository) {
				u.AddUser(&models.User{
					ID:         "user-pending",
					ExternalID: externalID,
					Email:      email,
					Role:       "student",
					IsActive:   true,
				})
				o.AddMembership(&models.OrganizationMembership{
					ID:             "mem-pending",
					OrganizationID: systemOrgID,
					UserID:         "user-pending",
					Role:           models.OrgRoleMember,
					IsPrimary:      true,
					InvitedAt:      time.Now(),
					// AcceptedAt deliberately nil.
				})
			},
			wantCreates:    1,
			wantPrimaryOrg: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := mocks.NewFakeUserRepository()
			orgRepo := mocks.NewFakeOrganizationMembershipRepository()
			tt.seed(userRepo, orgRepo)

			mgr := newLDAPTestAuthManager(userRepo, orgRepo)

			user, err := mgr.findOrProvisionLDAPUser(context.Background(), ldapInfo, email)
			require.NoError(t, err)
			require.NotNil(t, user)

			require.Equal(t, tt.wantCreates, orgRepo.CreateCount(),
				"unexpected number of membership Create calls")

			primaryOrg, err := orgRepo.GetPrimaryOrganization(context.Background(), user.ID)
			require.NoError(t, err)
			if tt.wantPrimaryOrg {
				require.NotNil(t, primaryOrg,
					"user must hold a primary org, otherwise the JWT carries no DefaultOrganizationID")
				require.Equal(t, systemOrgID, primaryOrg.ID)
			} else {
				require.Nil(t, primaryOrg)
			}

			// Every membership this login created must satisfy the predicates that
			// GetPrimaryOrganization and the tenant middleware rely on.
			for _, m := range orgRepo.Created {
				require.Equal(t, systemOrgID, m.OrganizationID)
				require.Equal(t, user.ID, m.UserID)
				require.Equal(t, models.OrgRoleMember, m.Role)
				require.True(t, m.IsPrimary, "GetPrimaryOrganization filters on is_primary = true")
				require.NotNil(t, m.AcceptedAt, "GetPrimaryOrganization filters on accepted_at IS NOT NULL")
				require.False(t, m.InvitedAt.IsZero(),
					"InvitedAt is passed straight into a NOT NULL TIMESTAMPTZ; zero would write 0001-01-01")
			}
		})
	}
}

// TestFindOrProvisionLDAPUser_MembershipFailureDoesNotBlockLogin pins the
// deliberate degradation: a membership repo that is broken must not lock users
// out. Login still succeeds; the user just has no default org until a later
// login repairs it.
func TestFindOrProvisionLDAPUser_MembershipFailureDoesNotBlockLogin(t *testing.T) {
	const (
		ldapUID    = "jdoe"
		externalID = "ldap:" + ldapUID
		email      = "jdoe@example.edu"
	)

	ldapInfo := &authldap.UserInfo{UID: ldapUID, Email: email, Role: "student"}

	tests := []struct {
		name        string
		injectErr   func(*mocks.FakeOrganizationMembershipRepository)
		wantCreates int
	}{
		{
			name: "Create fails",
			injectErr: func(o *mocks.FakeOrganizationMembershipRepository) {
				o.CreateErr = errors.New("unique constraint violation")
			},
			wantCreates: 1,
		},
		{
			name: "GetPrimaryOrganization fails",
			injectErr: func(o *mocks.FakeOrganizationMembershipRepository) {
				o.GetPrimaryOrganizationErr = errors.New("connection reset")
			},
			// The lookup guards the write, so a failed lookup must not blind-write.
			wantCreates: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := mocks.NewFakeUserRepository()
			orgRepo := mocks.NewFakeOrganizationMembershipRepository()
			tt.injectErr(orgRepo)

			mgr := newLDAPTestAuthManager(userRepo, orgRepo)

			user, err := mgr.findOrProvisionLDAPUser(context.Background(), ldapInfo, email)
			require.NoError(t, err, "membership provisioning must not fail the login")
			require.NotNil(t, user)
			require.Equal(t, externalID, user.ExternalID, "the user is still created and linked")
			require.Equal(t, tt.wantCreates, orgRepo.CreateCount())

			// The user row is persisted even though membership provisioning failed
			// — which is exactly why the next login has to retry.
			persisted, err := userRepo.GetByExternalID(context.Background(), externalID)
			require.NoError(t, err)
			require.NotNil(t, persisted)
		})
	}
}

// TestFindOrProvisionLDAPUser_RetriesAfterFailedCreate walks the actual bug
// across two logins: the first login persists the user but fails to write the
// membership, the second must repair it. This is the sequence the old code
// could not recover from.
func TestFindOrProvisionLDAPUser_RetriesAfterFailedCreate(t *testing.T) {
	const email = "jdoe@example.edu"
	ldapInfo := &authldap.UserInfo{UID: "jdoe", Email: email, Role: "student"}
	ctx := context.Background()

	userRepo := mocks.NewFakeUserRepository()
	orgRepo := mocks.NewFakeOrganizationMembershipRepository()
	mgr := newLDAPTestAuthManager(userRepo, orgRepo)

	// Login 1: the membership insert fails. The user is still persisted.
	orgRepo.CreateErr = errors.New("transient DB error")

	firstUser, err := mgr.findOrProvisionLDAPUser(ctx, ldapInfo, email)
	require.NoError(t, err)
	require.Equal(t, 1, orgRepo.CreateCount())

	primaryOrg, err := orgRepo.GetPrimaryOrganization(ctx, firstUser.ID)
	require.NoError(t, err)
	require.Nil(t, primaryOrg, "membership creation failed, so there is no primary org yet")

	// Login 2: the DB has recovered. The external-ID lookup now short-circuits
	// user creation — the branch that used to skip membership provisioning
	// entirely — but the membership must still be written.
	orgRepo.CreateErr = nil

	secondUser, err := mgr.findOrProvisionLDAPUser(ctx, ldapInfo, email)
	require.NoError(t, err)
	require.Equal(t, firstUser.ID, secondUser.ID, "the same user row is reused, not recreated")
	require.Equal(t, 2, orgRepo.CreateCount(), "the second login must retry the membership create")

	primaryOrg, err = orgRepo.GetPrimaryOrganization(ctx, secondUser.ID)
	require.NoError(t, err)
	require.NotNil(t, primaryOrg, "the second login must self-heal the missing membership")
	require.Equal(t, systemOrgID, primaryOrg.ID)

	// Login 3: now healthy — no further writes.
	thirdUser, err := mgr.findOrProvisionLDAPUser(ctx, ldapInfo, email)
	require.NoError(t, err)
	require.Equal(t, firstUser.ID, thirdUser.ID)
	require.Equal(t, 2, orgRepo.CreateCount(), "a healthy user must not be written to again")
}

// TestEnsureSystemOrgMembership_NilRepo guards the nil-repo no-op: deployments
// without a membership repo configured must not panic on LDAP login.
func TestEnsureSystemOrgMembership_NilRepo(t *testing.T) {
	mgr := newLDAPTestAuthManager(mocks.NewFakeUserRepository(), nil)

	require.NoError(t, mgr.ensureSystemOrgMembership(context.Background(), "user-123"))

	user, err := mgr.findOrProvisionLDAPUser(
		context.Background(),
		&authldap.UserInfo{UID: "jdoe", Email: "jdoe@example.edu", Role: "student"},
		"jdoe@example.edu",
	)
	require.NoError(t, err)
	require.NotNil(t, user)
}
