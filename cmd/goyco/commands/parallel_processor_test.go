package commands_test

import (
	"errors"
	"sync"
	"testing"

	"goyco/cmd/goyco/commands"
	"goyco/internal/database"
	"goyco/internal/repositories"
	"goyco/internal/testutils"

	"golang.org/x/crypto/bcrypt"
)

func TestParallelProcessor_CreateUsersInParallel(t *testing.T) {
	const successCount = 4

	tests := []struct {
		name        string
		count       int
		repoFactory func() repositories.UserRepository
		progress    *commands.ProgressIndicator
		validate    func(t *testing.T, got []database.User)
		wantErr     bool
	}{
		{
			name:  "creates users with required fields",
			count: successCount,
			repoFactory: func() repositories.UserRepository {
				base := testutils.NewMockUserRepository()
				return newFakeUserRepo(base, 0, nil)
			},
			progress: nil,
			validate: func(t *testing.T, got []database.User) {
				t.Helper()
				if len(got) != successCount {
					t.Fatalf("expected %d users, got %d", successCount, len(got))
				}
				usernames := make(map[string]bool)
				for i, user := range got {
					if user.Username == "" {
						t.Errorf("user %d expected non-empty username", i)
					}
					if len(user.Username) < 6 || user.Username[:5] != "user_" {
						t.Errorf("user %d username should start with 'user_', got %q", i, user.Username)
					}
					if usernames[user.Username] {
						t.Errorf("user %d duplicate username: %q", i, user.Username)
					}
					usernames[user.Username] = true

					if user.Email == "" {
						t.Errorf("user %d expected non-empty email", i)
					}
					if len(user.Email) < 20 || user.Email[:5] != "user_" || user.Email[len(user.Email)-12:] != "@goyco.local" {
						t.Errorf("user %d email should match pattern 'user_*@goyco.local', got %q", i, user.Email)
					}
					if !user.EmailVerified {
						t.Errorf("user %d expected EmailVerified to be true", i)
					}
					if user.ID == 0 {
						t.Errorf("user %d expected non-zero ID", i)
					}
					if user.Password == "" {
						t.Errorf("user %d expected hashed password to be populated", i)
					}
					if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("password123")); err != nil {
						t.Errorf("user %d password not hashed correctly: %v", i, err)
					}
					if user.CreatedAt.IsZero() {
						t.Errorf("user %d expected CreatedAt to be set", i)
					}
					if user.UpdatedAt.IsZero() {
						t.Errorf("user %d expected UpdatedAt to be set", i)
					}
				}
			},
		},
		{
			name:  "returns error when repository create fails",
			count: 3,
			repoFactory: func() repositories.UserRepository {
				base := testutils.NewMockUserRepository()
				return newFakeUserRepo(base, 1, errors.New("create failure"))
			},
			progress: nil,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := tt.repoFactory()
			p := commands.NewParallelProcessor()
			passwordHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
			if err != nil {
				t.Fatalf("failed to generate password hash: %v", err)
			}
			p.SetPasswordHash(string(passwordHash))
			got, gotErr := p.CreateUsersInParallel(repo, tt.count, tt.progress)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("CreateUsersInParallel() failed: %v", gotErr)
				}
				if got != nil {
					t.Error("expected nil result when error occurs")
				}
				return
			}
			if tt.wantErr {
				t.Fatal("CreateUsersInParallel() succeeded unexpectedly")
			}
			if tt.validate != nil {
				tt.validate(t, got)
			}
		})
	}
}

type fakeUserRepo struct {
	repositories.UserRepository
	mu     sync.Mutex
	failAt int
	err    error
	calls  int
}

func newFakeUserRepo(base repositories.UserRepository, failAt int, err error) *fakeUserRepo {
	return &fakeUserRepo{
		UserRepository: base,
		failAt:         failAt,
		err:            err,
	}
}

func (r *fakeUserRepo) Create(user *database.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.failAt > 0 && r.calls >= r.failAt {
		return r.err
	}
	return r.UserRepository.Create(user)
}
