package testutils

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"goyco/internal/database"
	"goyco/internal/repositories"

	"golang.org/x/crypto/bcrypt"
)

var loginIPCounter uint64

func GenerateTestIP() string {
	counter := atomic.AddUint64(&loginIPCounter, 1)
	octet := 100 + (counter % 155)
	return fmt.Sprintf("192.168.1.%d", octet)
}

type TestUser struct {
	ID            uint
	Username      string
	Email         string
	Password      string
	EmailVerified bool
}

type TestPost struct {
	ID       uint
	Title    string
	URL      string
	Content  string
	AuthorID uint
}

type AuthenticatedClient struct {
	Client       *http.Client
	Token        string
	RefreshToken string
	BaseURL      string
}

type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type LoginData struct {
	Token        string `json:"token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type LoginResponse struct {
	Success bool      `json:"success"`
	Message string    `json:"message"`
	Data    LoginData `json:"data"`
}

type PostResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		ID        uint   `json:"id"`
		Title     string `json:"title"`
		URL       string `json:"url"`
		Content   string `json:"content"`
		AuthorID  uint   `json:"author_id"`
		UpVotes   int    `json:"up_votes"`
		DownVotes int    `json:"down_votes"`
		Score     int    `json:"score"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	} `json:"data"`
}

type PostsListResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Posts  []Post `json:"posts"`
		Count  int    `json:"count"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	} `json:"data"`
}

type Post struct {
	ID        uint   `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Content   string `json:"content"`
	AuthorID  uint   `json:"author_id"`
	UpVotes   int    `json:"up_votes"`
	DownVotes int    `json:"down_votes"`
	Score     int    `json:"score"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Author    struct {
		ID            uint   `json:"id"`
		Username      string `json:"username"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Locked        bool   `json:"locked"`
		CreatedAt     string `json:"created_at"`
		UpdatedAt     string `json:"updated_at"`
	} `json:"author"`
}

type VoteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

type HealthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Status    string `json:"status"`
		Timestamp string `json:"timestamp"`
		Version   string `json:"version"`
		Services  struct {
			Database string `json:"database"`
			API      string `json:"api"`
		} `json:"services"`

		PingTime      string `json:"ping_time,omitempty"`
		DatabaseStats struct {
			TotalQueries    int64  `json:"total_queries,omitempty"`
			SlowQueries     int64  `json:"slow_queries,omitempty"`
			AverageDuration string `json:"average_duration,omitempty"`
			MaxDuration     string `json:"max_duration,omitempty"`
			ErrorCount      int64  `json:"error_count,omitempty"`
			LastQueryTime   string `json:"last_query_time,omitempty"`
		} `json:"database_stats"`
	} `json:"data"`
}

type MetricsResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Posts struct {
			TotalCount    int64   `json:"total_count"`
			TopPostsCount int     `json:"top_posts_count"`
			TotalScore    int     `json:"total_score"`
			AverageScore  float64 `json:"average_score"`
		} `json:"posts"`
		Users struct {
			TotalCount int64 `json:"total_count"`
		} `json:"users"`
		Votes struct {
			TotalCount     int64   `json:"total_count"`
			AveragePerPost float64 `json:"average_per_post"`
			Note           string  `json:"note"`
		} `json:"votes"`
		System struct {
			Timestamp string `json:"timestamp"`
			Version   string `json:"version"`
		} `json:"system"`

		Database struct {
			TotalQueries    int64  `json:"total_queries,omitempty"`
			SlowQueries     int64  `json:"slow_queries,omitempty"`
			AverageDuration string `json:"average_duration,omitempty"`
			MaxDuration     string `json:"max_duration,omitempty"`
			ErrorCount      int64  `json:"error_count,omitempty"`
			LastQueryTime   string `json:"last_query_time,omitempty"`
		} `json:"database"`
		Performance struct {
			RequestCount    int64  `json:"request_count,omitempty"`
			AverageResponse string `json:"average_response,omitempty"`
			MaxResponse     string `json:"max_response,omitempty"`
			ErrorCount      int64  `json:"error_count,omitempty"`
		} `json:"performance"`
	} `json:"data"`
}

type UserResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Users []struct {
			ID        uint   `json:"id"`
			Username  string `json:"username"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
		} `json:"users"`
		Count  int `json:"count"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	} `json:"data"`
}

type ProfileResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		ID            uint   `json:"id"`
		Username      string `json:"username"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Locked        bool   `json:"locked"`
		CreatedAt     string `json:"created_at"`
		UpdatedAt     string `json:"updated_at"`
	} `json:"data"`
}

type AccountDeletionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		DeletionToken string `json:"deletion_token"`
		ExpiresAt     string `json:"expires_at"`
	} `json:"data"`
}

func CreateE2ETestUser(t *testing.T, userRepo repositories.UserRepository, username, email, password string) *TestUser {
	t.Helper()

	normalizedEmail := strings.ToLower(strings.TrimSpace(email))

	user := &database.User{
		Username:      username,
		Email:         normalizedEmail,
		Password:      hashPassword(password),
		EmailVerified: true,
	}

	if err := userRepo.Create(user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	createdUser, err := userRepo.GetByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to fetch created user: %v", err)
	}

	if !createdUser.EmailVerified {
		now := time.Now()
		createdUser.EmailVerified = true
		createdUser.EmailVerifiedAt = &now
		if err := userRepo.Update(createdUser); err != nil {
			t.Fatalf("Failed to auto-verify test user email: %v", err)
		}

	}

	if !createdUser.EmailVerified {
		t.Fatalf("User email verification not set correctly. Expected true, got %v", createdUser.EmailVerified)
	}

	return &TestUser{
		ID:            createdUser.ID,
		Username:      createdUser.Username,
		Email:         createdUser.Email,
		Password:      password,
		EmailVerified: createdUser.EmailVerified,
	}
}

func LoginUserSafe(client *http.Client, baseURL, username, password string) (*AuthenticatedClient, error) {
	loginData := map[string]string{
		"username": username,
		"password": password,
	}

	loginBody, err := json.Marshal(loginData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal login data: %w", err)
	}

	request, err := http.NewRequest("POST", baseURL+"/api/auth/login", bytes.NewReader(loginBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create login request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	WithStandardHeaders(request)
	request.Header.Set("X-Forwarded-For", GenerateTestIP())

	resp, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to make login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		return nil, fmt.Errorf("login failed with status %d. Response: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	reader, err := decompressResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress response: %w", err)
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(reader).Decode(&loginResp); err != nil {
		return nil, fmt.Errorf("failed to decode login response: %w", err)
	}

	if !loginResp.Success {
		return nil, fmt.Errorf("login response indicates failure: %s", loginResp.Message)
	}

	accessToken := loginResp.Data.AccessToken
	if accessToken == "" {
		accessToken = loginResp.Data.Token
	}

	if accessToken == "" {
		return nil, fmt.Errorf("login response missing access token")
	}

	return &AuthenticatedClient{
		Client:       client,
		Token:        accessToken,
		RefreshToken: loginResp.Data.RefreshToken,
		BaseURL:      baseURL,
	}, nil
}

func LoginUser(t *testing.T, client *http.Client, baseURL, username, password string) *AuthenticatedClient {
	t.Helper()

	loginData := map[string]string{
		"username": username,
		"password": password,
	}

	loginBody, err := json.Marshal(loginData)
	if err != nil {
		t.Fatalf("Failed to marshal login data: %v", err)
	}

	request, err := http.NewRequest("POST", baseURL+"/api/auth/login", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("Failed to create login request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	WithStandardHeaders(request)
	request.Header.Set("X-Forwarded-For", GenerateTestIP())

	resp, err := client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make login request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		t.Fatalf("Login failed with status %d. Response: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	reader, err := decompressResponse(resp)
	if err != nil {
		t.Fatalf("Failed to decompress response: %v", err)
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(reader).Decode(&loginResp); err != nil {
		t.Fatalf("Failed to decode login response: %v", err)
	}

	if !loginResp.Success {
		t.Fatalf("Login response indicates failure: %s", loginResp.Message)
	}

	accessToken := loginResp.Data.AccessToken
	if accessToken == "" {
		accessToken = loginResp.Data.Token
	}

	if accessToken == "" {
		t.Fatalf("Login response missing access token")
	}

	return &AuthenticatedClient{
		Client:       client,
		Token:        accessToken,
		RefreshToken: loginResp.Data.RefreshToken,
		BaseURL:      baseURL,
	}
}

func CreateOversizedPayload() []byte {
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = 'A'
	}
	return data
}

func WithStandardHeaders(request *http.Request) {
	request.Header.Set("User-Agent", StandardUserAgent)
	request.Header.Set("Accept-Encoding", StandardAcceptEncoding)
}

func AssertPostInList(t *testing.T, posts *PostsListResponse, expectedPost *TestPost) {
	t.Helper()

	if len(posts.Data.Posts) == 0 {
		t.Errorf("Expected at least one post in response, got empty array")
		return
	}

	found := false
	for _, post := range posts.Data.Posts {
		if post.ID == expectedPost.ID && post.Title == expectedPost.Title {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected post with ID %d and title '%s' not found in posts list", expectedPost.ID, expectedPost.Title)
	}
}

func (ac *AuthenticatedClient) CreatePostSafe(title, url, content string) (*TestPost, error) {
	postData := map[string]string{
		"title":   title,
		"url":     url,
		"content": content,
	}

	postBody, err := json.Marshal(postData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal post data: %w", err)
	}

	request, err := http.NewRequest("POST", ac.BaseURL+"/api/posts", bytes.NewReader(postBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create post request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to make post request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("post creation failed with status %d", resp.StatusCode)
	}

	reader, err := decompressResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress response: %w", err)
	}

	var postResp PostResponse
	if err := json.NewDecoder(reader).Decode(&postResp); err != nil {
		return nil, fmt.Errorf("failed to decode post response: %w", err)
	}

	if !postResp.Success {
		return nil, fmt.Errorf("post creation response indicates failure: %s", postResp.Message)
	}

	return &TestPost{
		ID:       postResp.Data.ID,
		Title:    postResp.Data.Title,
		URL:      postResp.Data.URL,
		Content:  postResp.Data.Content,
		AuthorID: postResp.Data.AuthorID,
	}, nil
}

func (ac *AuthenticatedClient) VoteOnPostSafe(postID uint, voteType string) (*VoteResponse, error) {
	voteData := map[string]string{
		"type": voteType,
	}

	voteBody, err := json.Marshal(voteData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal vote data: %w", err)
	}

	url := fmt.Sprintf("%s/api/posts/%d/vote", ac.BaseURL, postID)
	request, err := http.NewRequest("POST", url, bytes.NewReader(voteBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create vote request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to make vote request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vote failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read vote response: %w", err)
	}

	var voteResp VoteResponse
	if len(body) > 0 {
		if err := json.Unmarshal(body, &voteResp); err != nil {
			voteResp = VoteResponse{
				Success: false,
				Message: string(bytes.TrimSpace(body)),
			}
		}
	}

	if !voteResp.Success {
		return nil, fmt.Errorf("vote response indicates failure: %s", voteResp.Message)
	}

	return &voteResp, nil
}

func (ac *AuthenticatedClient) SearchPostsSafe(query string) (*PostsListResponse, error) {
	url := fmt.Sprintf("%s/api/posts/search?q=%s", ac.BaseURL, query)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to make search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search posts failed with status %d", resp.StatusCode)
	}

	reader, err := decompressResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress response: %w", err)
	}

	var searchResp PostsListResponse
	if err := json.NewDecoder(reader).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	if !searchResp.Success {
		return nil, fmt.Errorf("search posts response indicates failure: %s", searchResp.Message)
	}

	return &searchResp, nil
}

func (ac *AuthenticatedClient) CreatePost(t *testing.T, title, url, content string) *TestPost {
	t.Helper()

	postData := map[string]string{
		"title":   title,
		"url":     url,
		"content": content,
	}

	postBody, err := json.Marshal(postData)
	if err != nil {
		t.Fatalf("Failed to marshal post data: %v", err)
	}

	request, err := http.NewRequest("POST", ac.BaseURL+"/api/posts", bytes.NewReader(postBody))
	if err != nil {
		t.Fatalf("Failed to create post request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make post request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Post creation failed with status %d", resp.StatusCode)
	}

	reader, err := decompressResponse(resp)
	if err != nil {
		t.Fatalf("Failed to decompress response: %v", err)
	}

	var postResp PostResponse
	if err := json.NewDecoder(reader).Decode(&postResp); err != nil {
		t.Fatalf("Failed to decode post response: %v", err)
	}

	if !postResp.Success {
		t.Fatalf("Post creation response indicates failure: %s", postResp.Message)
	}

	return &TestPost{
		ID:       postResp.Data.ID,
		Title:    postResp.Data.Title,
		URL:      postResp.Data.URL,
		Content:  postResp.Data.Content,
		AuthorID: postResp.Data.AuthorID,
	}
}

func (ac *AuthenticatedClient) VoteOnPost(t *testing.T, postID uint, voteType string) *VoteResponse {
	t.Helper()

	voteResp, statusCode := ac.VoteOnPostRaw(t, postID, voteType)
	if statusCode != http.StatusOK {
		t.Fatalf("Vote failed with status %d", statusCode)
	}

	if !voteResp.Success {
		t.Fatalf("Vote response indicates failure: %s", voteResp.Message)
	}

	return voteResp
}

func (ac *AuthenticatedClient) VoteOnPostRaw(t *testing.T, postID uint, voteType string) (*VoteResponse, int) {
	t.Helper()

	voteData := map[string]string{
		"type": voteType,
	}

	voteBody, err := json.Marshal(voteData)
	if err != nil {
		t.Fatalf("Failed to marshal vote data: %v", err)
	}

	url := fmt.Sprintf("%s/api/posts/%d/vote", ac.BaseURL, postID)
	request, err := http.NewRequest("POST", url, bytes.NewReader(voteBody))
	if err != nil {
		t.Fatalf("Failed to create vote request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make vote request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read vote response: %v", err)
	}

	var voteResp VoteResponse
	if len(body) > 0 {
		if err := json.Unmarshal(body, &voteResp); err != nil {
			voteResp = VoteResponse{
				Success: false,
				Message: string(bytes.TrimSpace(body)),
			}
		}
	}

	return &voteResp, resp.StatusCode
}

func (ac *AuthenticatedClient) GetPosts(t *testing.T) *PostsListResponse {
	t.Helper()

	request, err := http.NewRequest("GET", ac.BaseURL+"/api/posts", nil)
	if err != nil {
		t.Fatalf("Failed to create posts request: %v", err)
	}
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make posts request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Get posts failed with status %d", resp.StatusCode)
	}

	reader, err := decompressResponse(resp)
	if err != nil {
		t.Fatalf("Failed to decompress response: %v", err)
	}

	var postsResp PostsListResponse
	if err := json.NewDecoder(reader).Decode(&postsResp); err != nil {
		t.Fatalf("Failed to decode posts response: %v", err)
	}

	if !postsResp.Success {
		t.Fatalf("Get posts response indicates failure: %s", postsResp.Message)
	}

	return &postsResp
}

func (ac *AuthenticatedClient) SearchPosts(t *testing.T, query string) *PostsListResponse {
	t.Helper()

	url := fmt.Sprintf("%s/api/posts/search?q=%s", ac.BaseURL, query)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("Failed to create search request: %v", err)
	}
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make search request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Search posts failed with status %d", resp.StatusCode)
	}

	reader, err := decompressResponse(resp)
	if err != nil {
		t.Fatalf("Failed to decompress response: %v", err)
	}

	var searchResp PostsListResponse
	if err := json.NewDecoder(reader).Decode(&searchResp); err != nil {
		t.Fatalf("Failed to decode search response: %v", err)
	}

	if !searchResp.Success {
		t.Fatalf("Search posts response indicates failure: %s", searchResp.Message)
	}

	return &searchResp
}

func (ac *AuthenticatedClient) Logout(t *testing.T) {
	t.Helper()

	request, err := http.NewRequest("POST", ac.BaseURL+"/api/auth/logout", nil)
	if err != nil {
		t.Fatalf("Failed to create logout request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make logout request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Logout failed with status %d", resp.StatusCode)
	}
}

func (ac *AuthenticatedClient) RefreshAccessToken(t *testing.T, ipAddress ...string) (string, int) {
	t.Helper()

	var ip string
	if len(ipAddress) > 0 {
		ip = ipAddress[0]
	}

	newAccessToken, newRefreshToken, statusCode := RefreshTokenWithIP(t, ac.Client, ac.BaseURL, ac.RefreshToken, ip)
	if statusCode == http.StatusOK {
		ac.Token = newAccessToken
		if newRefreshToken != "" {
			ac.RefreshToken = newRefreshToken
		}
	}

	return newAccessToken, statusCode
}

func (ac *AuthenticatedClient) RevokeToken(t *testing.T, refreshToken string) int {
	t.Helper()
	return RevokeToken(t, ac.Client, ac.BaseURL, refreshToken, ac.Token)
}

func (ac *AuthenticatedClient) RevokeAllTokens(t *testing.T) int {
	t.Helper()
	return RevokeAllTokens(t, ac.Client, ac.BaseURL, ac.Token)
}

func (ac *AuthenticatedClient) ConfirmAccountDeletion(t *testing.T, token string, deletePosts bool) int {
	t.Helper()
	return ConfirmAccountDeletion(t, ac.Client, ac.BaseURL, token, deletePosts)
}

func (ac *AuthenticatedClient) GetProfile(t *testing.T) *ProfileResponse {
	t.Helper()

	request, err := http.NewRequest("GET", ac.BaseURL+"/api/auth/me", nil)
	if err != nil {
		t.Fatalf("Failed to create profile request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make profile request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Get profile failed with status %d", resp.StatusCode)
	}

	reader, err := decompressResponse(resp)
	if err != nil {
		t.Fatalf("Failed to decompress response: %v", err)
	}

	var profileResp ProfileResponse
	if err := json.NewDecoder(reader).Decode(&profileResp); err != nil {
		t.Fatalf("Failed to decode profile response: %v", err)
	}

	if !profileResp.Success {
		t.Fatalf("Get profile response indicates failure: %s", profileResp.Message)
	}

	return &profileResp
}

func (ac *AuthenticatedClient) UpdateUsername(t *testing.T, newUsername string) {
	t.Helper()

	updateData := map[string]string{
		"username": newUsername,
	}

	updateBody, err := json.Marshal(updateData)
	if err != nil {
		t.Fatalf("Failed to marshal username update data: %v", err)
	}

	request, err := http.NewRequest("PUT", ac.BaseURL+"/api/auth/username", bytes.NewReader(updateBody))
	if err != nil {
		t.Fatalf("Failed to create username update request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make username update request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Username update failed with status %d", resp.StatusCode)
	}
}

func (ac *AuthenticatedClient) UpdatePassword(t *testing.T, currentPassword, newPassword string) {
	t.Helper()

	updateData := map[string]string{
		"current_password": currentPassword,
		"new_password":     newPassword,
	}

	updateBody, err := json.Marshal(updateData)
	if err != nil {
		t.Fatalf("Failed to marshal password update data: %v", err)
	}

	request, err := http.NewRequest("PUT", ac.BaseURL+"/api/auth/password", bytes.NewReader(updateBody))
	if err != nil {
		t.Fatalf("Failed to create password update request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make password update request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Password update failed with status %d", resp.StatusCode)
	}
}

func (ac *AuthenticatedClient) RegisterUser(t *testing.T, username, email, password string) *LoginResponse {
	t.Helper()

	registerData := map[string]string{
		"username": username,
		"email":    email,
		"password": password,
	}

	registerBody, err := json.Marshal(registerData)
	if err != nil {
		t.Fatalf("Failed to marshal register data: %v", err)
	}

	request, err := http.NewRequest("POST", ac.BaseURL+"/api/auth/register", bytes.NewReader(registerBody))
	if err != nil {
		t.Fatalf("Failed to create register request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make register request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Registration failed with status %d", resp.StatusCode)
	}

	reader, err := decompressResponse(resp)
	if err != nil {
		t.Fatalf("Failed to decompress response: %v", err)
	}

	var registerResp LoginResponse
	if err := json.NewDecoder(reader).Decode(&registerResp); err != nil {
		t.Fatalf("Failed to decode register response: %v", err)
	}

	if !registerResp.Success {
		t.Fatalf("Registration response indicates failure: %s", registerResp.Message)
	}

	return &registerResp
}

func (ac *AuthenticatedClient) UpdateEmail(t *testing.T, newEmail string) {
	t.Helper()

	updateData := map[string]string{
		"email": newEmail,
	}

	updateBody, err := json.Marshal(updateData)
	if err != nil {
		t.Fatalf("Failed to marshal email update data: %v", err)
	}

	request, err := http.NewRequest("PUT", ac.BaseURL+"/api/auth/email", bytes.NewReader(updateBody))
	if err != nil {
		t.Fatalf("Failed to create email update request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make email update request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Email update failed with status %d", resp.StatusCode)
	}
}

func (ac *AuthenticatedClient) UpdatePost(t *testing.T, postID uint, title, url, content string) *TestPost {
	t.Helper()

	updateData := map[string]string{
		"title":   title,
		"url":     url,
		"content": content,
	}

	updateBody, err := json.Marshal(updateData)
	if err != nil {
		t.Fatalf("Failed to marshal post update data: %v", err)
	}

	postURL := fmt.Sprintf("%s/api/posts/%d", ac.BaseURL, postID)
	request, err := http.NewRequest("PUT", postURL, bytes.NewReader(updateBody))
	if err != nil {
		t.Fatalf("Failed to create post update request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make post update request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Post update failed with status %d", resp.StatusCode)
	}

	var postResp PostResponse
	if err := json.NewDecoder(resp.Body).Decode(&postResp); err != nil {
		t.Fatalf("Failed to decode post update response: %v", err)
	}

	if !postResp.Success {
		t.Fatalf("Post update response indicates failure: %s", postResp.Message)
	}

	return &TestPost{
		ID:       postResp.Data.ID,
		Title:    postResp.Data.Title,
		URL:      postResp.Data.URL,
		Content:  postResp.Data.Content,
		AuthorID: postResp.Data.AuthorID,
	}
}

func (ac *AuthenticatedClient) DeletePost(t *testing.T, postID uint) {
	t.Helper()

	url := fmt.Sprintf("%s/api/posts/%d", ac.BaseURL, postID)
	request, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		t.Fatalf("Failed to create post delete request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make post delete request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Post delete failed with status %d", resp.StatusCode)
	}
}

func (ac *AuthenticatedClient) RemoveVote(t *testing.T, postID uint) {
	t.Helper()

	url := fmt.Sprintf("%s/api/posts/%d/vote", ac.BaseURL, postID)
	request, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		t.Fatalf("Failed to create vote removal request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make vote removal request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Vote removal failed with status %d", resp.StatusCode)
	}
}

func (ac *AuthenticatedClient) GetUserVote(t *testing.T, postID uint) *VoteResponse {
	t.Helper()

	url := fmt.Sprintf("%s/api/posts/%d/vote", ac.BaseURL, postID)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("Failed to create get vote request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make get vote request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Get vote failed with status %d", resp.StatusCode)
	}

	var voteResp VoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&voteResp); err != nil {
		t.Fatalf("Failed to decode vote response: %v", err)
	}

	return &voteResp
}

func (ac *AuthenticatedClient) GetPostVotes(t *testing.T, postID uint) *VoteResponse {
	t.Helper()

	url := fmt.Sprintf("%s/api/posts/%d/votes", ac.BaseURL, postID)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("Failed to create get post votes request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make get post votes request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Get post votes failed with status %d", resp.StatusCode)
	}

	var voteResp VoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&voteResp); err != nil {
		t.Fatalf("Failed to decode post votes response: %v", err)
	}

	return &voteResp
}

func (ac *AuthenticatedClient) GetUsers(t *testing.T) *UserResponse {
	t.Helper()

	request, err := http.NewRequest("GET", ac.BaseURL+"/api/users", nil)
	if err != nil {
		t.Fatalf("Failed to create get users request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	response, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make get users request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("Get users failed with status %d", response.StatusCode)
	}

	reader, err := decompressResponse(response)
	if err != nil {
		t.Fatalf("Failed to decompress response: %v", err)
	}

	var usersResponse UserResponse
	if err := json.NewDecoder(reader).Decode(&usersResponse); err != nil {
		t.Fatalf("Failed to decode users response: %v", err)
	}

	if !usersResponse.Success {
		t.Fatalf("Get users response indicates failure: %s", usersResponse.Message)
	}

	return &usersResponse
}

func (ac *AuthenticatedClient) RequestAccountDeletion(t *testing.T) *AccountDeletionResponse {
	t.Helper()

	request, err := http.NewRequest("DELETE", ac.BaseURL+"/api/auth/account", nil)
	if err != nil {
		t.Fatalf("Failed to create account deletion request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	response, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make account deletion request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		bodyBytes := make([]byte, 1024)
		n, _ := response.Body.Read(bodyBytes)
		t.Fatalf("Account deletion request failed with status %d. Response: %s", response.StatusCode, string(bodyBytes[:n]))
	}

	reader, err := decompressResponse(response)
	if err != nil {
		t.Fatalf("Failed to decompress response: %v", err)
	}

	var deletionResponse AccountDeletionResponse
	if err := json.NewDecoder(reader).Decode(&deletionResponse); err != nil {
		t.Fatalf("Failed to decode account deletion response: %v", err)
	}

	if !deletionResponse.Success {
		t.Fatalf("Account deletion response indicates failure: %s", deletionResponse.Message)
	}

	return &deletionResponse
}

func ConfirmAccountDeletion(t *testing.T, client *http.Client, baseURL, token string, deletePosts bool) int {
	t.Helper()

	requestData := map[string]any{
		"token":        token,
		"delete_posts": deletePosts,
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		t.Fatalf("Failed to marshal account deletion confirmation request: %v", err)
	}

	request, err := http.NewRequest("POST", baseURL+"/api/auth/account/confirm", bytes.NewReader(requestBody))
	if err != nil {
		t.Fatalf("Failed to create account deletion confirmation request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	WithStandardHeaders(request)

	resp, err := client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make account deletion confirmation request: %v", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

func ResendVerificationEmail(t *testing.T, client *http.Client, baseURL, email string) int {
	t.Helper()

	requestData := map[string]string{
		"email": email,
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		t.Fatalf("Failed to marshal resend verification request: %v", err)
	}

	request, err := http.NewRequest("POST", baseURL+"/api/auth/resend-verification", bytes.NewReader(requestBody))
	if err != nil {
		t.Fatalf("Failed to create resend verification request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	WithStandardHeaders(request)

	resp, err := client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make resend verification request: %v", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

func hashPassword(password string) string {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		panic(fmt.Sprintf("Failed to hash password: %v", err))
	}
	return string(hashedPassword)
}

func decompressResponse(resp *http.Response) (io.Reader, error) {
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		return gzReader, nil
	}
	return resp.Body, nil
}

func GetHealth(t *testing.T, client *http.Client, baseURL string) *HealthResponse {
	t.Helper()

	request, err := http.NewRequest("GET", baseURL+"/health", nil)
	if err != nil {
		t.Fatalf("Failed to create health request: %v", err)
	}
	WithStandardHeaders(request)

	resp, err := client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make health request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Health check failed with status %d", resp.StatusCode)
	}

	reader, err := decompressResponse(resp)
	if err != nil {
		t.Fatalf("Failed to decompress response: %v", err)
	}

	var healthResp HealthResponse
	if err := json.NewDecoder(reader).Decode(&healthResp); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	if !healthResp.Success {
		t.Fatalf("Health response indicates failure: %s", healthResp.Message)
	}

	return &healthResp
}

func GetMetrics(t *testing.T, client *http.Client, baseURL string) *MetricsResponse {
	t.Helper()

	request, err := http.NewRequest("GET", baseURL+"/metrics", nil)
	if err != nil {
		t.Fatalf("Failed to create metrics request: %v", err)
	}
	WithStandardHeaders(request)

	resp, err := client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make metrics request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Metrics request failed with status %d", resp.StatusCode)
	}

	reader, err := decompressResponse(resp)
	if err != nil {
		t.Fatalf("Failed to decompress response: %v", err)
	}

	var metricsResp MetricsResponse
	if err := json.NewDecoder(reader).Decode(&metricsResp); err != nil {
		t.Fatalf("Failed to decode metrics response: %v", err)
	}

	if !metricsResp.Success {
		t.Fatalf("Metrics response indicates failure: %s", metricsResp.Message)
	}

	return &metricsResp
}

func (ac *AuthenticatedClient) UpdatePostExpectStatus(t *testing.T, postID uint, title, url, content string) int {
	t.Helper()

	updateData := map[string]string{
		"title":   title,
		"url":     url,
		"content": content,
	}

	updateBody, err := json.Marshal(updateData)
	if err != nil {
		t.Fatalf("Failed to marshal post update data: %v", err)
	}

	postURL := fmt.Sprintf("%s/api/posts/%d", ac.BaseURL, postID)
	request, err := http.NewRequest("PUT", postURL, bytes.NewReader(updateBody))
	if err != nil {
		t.Fatalf("Failed to create post update request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make post update request: %v", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

func (ac *AuthenticatedClient) DeletePostExpectStatus(t *testing.T, postID uint) int {
	t.Helper()

	url := fmt.Sprintf("%s/api/posts/%d", ac.BaseURL, postID)
	request, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		t.Fatalf("Failed to create post delete request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make post delete request: %v", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

func (ac *AuthenticatedClient) UpdateEmailExpectStatus(t *testing.T, newEmail string) int {
	t.Helper()

	updateData := map[string]string{
		"email": newEmail,
	}

	updateBody, err := json.Marshal(updateData)
	if err != nil {
		t.Fatalf("Failed to marshal email update data: %v", err)
	}

	request, err := http.NewRequest("PUT", ac.BaseURL+"/api/auth/email", bytes.NewReader(updateBody))
	if err != nil {
		t.Fatalf("Failed to create email update request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make email update request: %v", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

func (ac *AuthenticatedClient) UpdateUsernameExpectStatus(t *testing.T, newUsername string) int {
	t.Helper()

	updateData := map[string]string{
		"username": newUsername,
	}

	updateBody, err := json.Marshal(updateData)
	if err != nil {
		t.Fatalf("Failed to marshal username update data: %v", err)
	}

	request, err := http.NewRequest("PUT", ac.BaseURL+"/api/auth/username", bytes.NewReader(updateBody))
	if err != nil {
		t.Fatalf("Failed to create username update request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make username update request: %v", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

func AssertVoteData(t *testing.T, voteResp *VoteResponse) map[string]any {
	t.Helper()
	data, ok := voteResp.Data.(map[string]any)
	if !ok {
		t.Fatalf("Expected vote data to be a map, got %T", voteResp.Data)
	}
	return data
}

func (ac *AuthenticatedClient) UpdatePasswordExpectStatus(t *testing.T, currentPassword, newPassword string) int {
	t.Helper()

	updateData := map[string]string{
		"current_password": currentPassword,
		"new_password":     newPassword,
	}

	updateBody, err := json.Marshal(updateData)
	if err != nil {
		t.Fatalf("Failed to marshal password update data: %v", err)
	}

	request, err := http.NewRequest("PUT", ac.BaseURL+"/api/auth/password", bytes.NewReader(updateBody))
	if err != nil {
		t.Fatalf("Failed to create password update request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+ac.Token)
	WithStandardHeaders(request)

	resp, err := ac.Client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make password update request: %v", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

func RequestPasswordReset(t *testing.T, client *http.Client, baseURL, usernameOrEmail, ipAddress string) int {
	t.Helper()

	requestData := map[string]string{
		"username_or_email": usernameOrEmail,
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		t.Fatalf("Failed to marshal password reset request: %v", err)
	}

	request, err := http.NewRequest("POST", baseURL+"/api/auth/forgot-password", bytes.NewReader(requestBody))
	if err != nil {
		t.Fatalf("Failed to create password reset request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	WithStandardHeaders(request)
	if ipAddress != "" {
		request.Header.Set("X-Forwarded-For", ipAddress)
	}

	resp, err := client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make password reset request: %v", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

func ResetPassword(t *testing.T, client *http.Client, baseURL, token, newPassword, ipAddress string) int {
	t.Helper()

	requestData := map[string]string{
		"token":        token,
		"new_password": newPassword,
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		t.Fatalf("Failed to marshal password reset request: %v", err)
	}

	request, err := http.NewRequest("POST", baseURL+"/api/auth/reset-password", bytes.NewReader(requestBody))
	if err != nil {
		t.Fatalf("Failed to create password reset request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	WithStandardHeaders(request)
	if ipAddress != "" {
		request.Header.Set("X-Forwarded-For", ipAddress)
	}

	resp, err := client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make password reset request: %v", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

func RefreshToken(t *testing.T, client *http.Client, baseURL, refreshToken string) (accessToken string, returnedRefreshToken string, statusCode int) {
	return RefreshTokenWithIP(t, client, baseURL, refreshToken, "")
}

func RefreshTokenWithIP(t *testing.T, client *http.Client, baseURL, refreshToken, ipAddress string) (accessToken string, returnedRefreshToken string, statusCode int) {
	t.Helper()

	requestData := map[string]string{
		"refresh_token": refreshToken,
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		t.Fatalf("Failed to marshal refresh token request: %v", err)
	}

	request, err := http.NewRequest("POST", baseURL+"/api/auth/refresh", bytes.NewReader(requestBody))
	if err != nil {
		t.Fatalf("Failed to create refresh token request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	WithStandardHeaders(request)
	if ipAddress != "" {
		request.Header.Set("X-Forwarded-For", ipAddress)
	}

	resp, err := client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make refresh token request: %v", err)
	}
	defer resp.Body.Close()

	reader, err := decompressResponse(resp)
	if err != nil {
		reader = resp.Body
	}

	var refreshResp LoginResponse
	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read refresh token response: %v", err)
	}

	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(bodyBytes, &refreshResp); err != nil {
			t.Fatalf("Failed to decode refresh token response: %v. Body: %s", err, string(bodyBytes))
		}

		accessToken = refreshResp.Data.AccessToken
		if accessToken == "" {
			accessToken = refreshResp.Data.Token
		}
		returnedRefreshToken = refreshResp.Data.RefreshToken
	}

	return accessToken, returnedRefreshToken, resp.StatusCode
}

func RevokeToken(t *testing.T, client *http.Client, baseURL, refreshToken, accessToken string) int {
	t.Helper()

	requestData := map[string]string{
		"refresh_token": refreshToken,
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		t.Fatalf("Failed to marshal revoke token request: %v", err)
	}

	request, err := http.NewRequest("POST", baseURL+"/api/auth/revoke", bytes.NewReader(requestBody))
	if err != nil {
		t.Fatalf("Failed to create revoke token request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	WithStandardHeaders(request)

	resp, err := client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make revoke token request: %v", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

func RevokeAllTokens(t *testing.T, client *http.Client, baseURL, accessToken string) int {
	t.Helper()

	request, err := http.NewRequest("POST", baseURL+"/api/auth/revoke-all", nil)
	if err != nil {
		t.Fatalf("Failed to create revoke all tokens request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	WithStandardHeaders(request)

	resp, err := client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make revoke all tokens request: %v", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode
}
