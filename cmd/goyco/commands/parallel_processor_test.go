package commands_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"goyco/cmd/goyco/commands"
	"goyco/internal/database"
	"goyco/internal/repositories"
	"goyco/internal/testutils"
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
			name:  "creates users with deterministic fields",
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
				for i, user := range got {
					expectedUsername := fmt.Sprintf("user_%d", i+1)
					expectedEmail := fmt.Sprintf("user_%d@goyco.local", i+1)
					if user.Username != expectedUsername {
						t.Errorf("user %d username mismatch: got %q want %q", i, user.Username, expectedUsername)
					}
					if user.Email != expectedEmail {
						t.Errorf("user %d email mismatch: got %q want %q", i, user.Email, expectedEmail)
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
