package commands

import (
	"context"
	cryptoRand "crypto/rand"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"goyco/internal/database"
	"goyco/internal/repositories"
)

type ParallelProcessor struct {
	maxWorkers   int
	timeout      time.Duration
	passwordHash string
	randSource   *rand.Rand
	randMu       sync.Mutex
}

func NewParallelProcessor() *ParallelProcessor {
	maxWorkers := max(min(runtime.NumCPU(), 8), 2)

	seed := time.Now().UnixNano()
	seedBytes := make([]byte, 8)
	if _, err := cryptoRand.Read(seedBytes); err == nil {
		seed = int64(seedBytes[0])<<56 | int64(seedBytes[1])<<48 | int64(seedBytes[2])<<40 | int64(seedBytes[3])<<32 |
			int64(seedBytes[4])<<24 | int64(seedBytes[5])<<16 | int64(seedBytes[6])<<8 | int64(seedBytes[7])
	}

	return &ParallelProcessor{
		maxWorkers: maxWorkers,
		timeout:    60 * time.Second,
		randSource: rand.New(rand.NewSource(seed)),
	}
}

func (p *ParallelProcessor) SetPasswordHash(hash string) {
	p.passwordHash = hash
}

func (p *ParallelProcessor) CreateUsersInParallel(userRepo repositories.UserRepository, count int, progress *ProgressIndicator) ([]database.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	results := make(chan userResult, count)
	errors := make(chan error, count)

	semaphore := make(chan struct{}, p.maxWorkers)
	var wg sync.WaitGroup

	for i := range count {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				errors <- ctx.Err()
				return
			}
			defer func() { <-semaphore }()

			user, err := p.createSingleUser(userRepo, index+1)
			if err != nil {
				errors <- fmt.Errorf("create user %d: %w", index+1, err)
				return
			}

			results <- userResult{user: user, index: index}
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	users := make([]database.User, count)
	completed := 0
	firstError := make(chan error, 1)

	go func() {
		for err := range errors {
			if err != nil {
				select {
				case firstError <- err:
				default:
				}
				return
			}
		}
	}()

	for completed < count {
		select {
		case result, ok := <-results:
			if !ok {
				return users, nil
			}
			users[result.index] = result.user
			completed++
			if progress != nil {
				progress.Update(completed)
			}
		case err := <-firstError:
			return nil, err
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout creating users: %w", ctx.Err())
		}
	}

	return users, nil
}

func (p *ParallelProcessor) CreatePostsInParallel(postRepo repositories.PostRepository, authorID uint, count int, progress *ProgressIndicator) ([]database.Post, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	results := make(chan postResult, count)
	errors := make(chan error, count)

	semaphore := make(chan struct{}, p.maxWorkers)
	var wg sync.WaitGroup

	for i := range count {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				errors <- ctx.Err()
				return
			}
			defer func() { <-semaphore }()

			post, err := p.createSinglePost(postRepo, authorID, index+1)
			if err != nil {
				errors <- fmt.Errorf("create post %d: %w", index+1, err)
				return
			}

			results <- postResult{post: post, index: index}
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	posts := make([]database.Post, count)
	completed := 0
	firstError := make(chan error, 1)

	go func() {
		for err := range errors {
			if err != nil {
				select {
				case firstError <- err:
				default:
				}
				return
			}
		}
	}()

	for completed < count {
		select {
		case result, ok := <-results:
			if !ok {
				return posts, nil
			}
			posts[result.index] = result.post
			completed++
			if progress != nil {
				progress.Update(completed)
			}
		case err := <-firstError:
			return nil, err
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout creating posts: %w", ctx.Err())
		}
	}

	return posts, nil
}

func (p *ParallelProcessor) CreateVotesInParallel(voteRepo repositories.VoteRepository, users []database.User, posts []database.Post, avgVotesPerPost int, progress *ProgressIndicator) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	results := make(chan voteResult, len(posts))
	errors := make(chan error, len(posts))

	semaphore := make(chan struct{}, p.maxWorkers)
	var wg sync.WaitGroup

	for i, post := range posts {
		wg.Add(1)
		go func(index int, post database.Post) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				errors <- ctx.Err()
				return
			}
			defer func() { <-semaphore }()

			votes, err := p.createVotesForPost(voteRepo, users, post, avgVotesPerPost)
			if err != nil {
				errors <- fmt.Errorf("create votes for post %d: %w", post.ID, err)
				return
			}

			results <- voteResult{votes: votes, index: index}
		}(i, post)
	}

	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	totalVotes := 0
	completed := 0
	firstError := make(chan error, 1)

	go func() {
		for err := range errors {
			if err != nil {
				select {
				case firstError <- err:
				default:
				}
				return
			}
		}
	}()

	for completed < len(posts) {
		select {
		case result, ok := <-results:
			if !ok {
				return totalVotes, nil
			}
			totalVotes += result.votes
			completed++
			if progress != nil {
				progress.Update(completed)
			}
		case err := <-firstError:
			return 0, err
		case <-ctx.Done():
			return 0, fmt.Errorf("timeout creating votes: %w", ctx.Err())
		}
	}

	return totalVotes, nil
}

func (p *ParallelProcessor) UpdatePostScoresInParallel(postRepo repositories.PostRepository, voteRepo repositories.VoteRepository, posts []database.Post, progress *ProgressIndicator) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	errors := make(chan error, len(posts))

	semaphore := make(chan struct{}, p.maxWorkers)
	var wg sync.WaitGroup

	for i, post := range posts {
		wg.Add(1)
		go func(index int, post database.Post) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				errors <- ctx.Err()
				return
			}
			defer func() { <-semaphore }()

			err := p.updateSinglePostScore(postRepo, voteRepo, post)
			if err != nil {
				errors <- fmt.Errorf("update post %d scores: %w", post.ID, err)
				return
			}

			if progress != nil {
				progress.Update(index + 1)
			}
		}(i, post)
	}

	go func() {
		wg.Wait()
		close(errors)
	}()

	for err := range errors {
		if err != nil {
			return err
		}
	}

	return nil
}

type userResult struct {
	user  database.User
	index int
}

type postResult struct {
	post  database.Post
	index int
}

type voteResult struct {
	votes int
	index int
}

func (p *ParallelProcessor) generateRandomIdentifier() string {
	const length = 12
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	identifier := make([]byte, length)
	p.randMu.Lock()
	for i := range identifier {
		identifier[i] = chars[p.randSource.Intn(len(chars))]
	}
	p.randMu.Unlock()
	return string(identifier)
}

func (p *ParallelProcessor) createSingleUser(userRepo repositories.UserRepository, index int) (database.User, error) {
	randomID := p.generateRandomIdentifier()
	username := fmt.Sprintf("user_%s", randomID)
	email := fmt.Sprintf("user_%s@goyco.local", randomID)

	const maxRetries = 10
	for range maxRetries {
		user, err := userRepo.GetByEmail(email)
		if err == nil {
			return *user, nil
		}

		user = &database.User{
			Username:      username,
			Email:         email,
			Password:      p.passwordHash,
			EmailVerified: true,
		}

		if err := userRepo.Create(user); err != nil {
			randomID = p.generateRandomIdentifier()
			username = fmt.Sprintf("user_%s", randomID)
			email = fmt.Sprintf("user_%s@goyco.local", randomID)
			continue
		}

		return *user, nil
	}

	return database.User{}, fmt.Errorf("failed to create user after %d attempts", maxRetries)
}

func (p *ParallelProcessor) createSinglePost(postRepo repositories.PostRepository, authorID uint, index int) (database.Post, error) {
	sampleTitles := []string{
		"Amazing JavaScript Framework",
		"Python Best Practices",
		"Go Performance Tips",
		"Database Optimization",
		"Web Security Guide",
		"Machine Learning Basics",
		"Cloud Architecture",
		"DevOps Automation",
		"API Design Patterns",
		"Frontend Optimization",
		"Backend Scaling",
		"Container Orchestration",
		"Microservices Architecture",
		"Testing Strategies",
		"Code Review Process",
		"Version Control Best Practices",
		"Continuous Integration",
		"Monitoring and Alerting",
		"Error Handling Patterns",
		"Data Structures Explained",
	}

	sampleDomains := []string{
		"example.com",
		"techblog.org",
		"devguide.net",
		"programming.io",
		"codeexamples.com",
		"tutorialhub.org",
		"bestpractices.dev",
		"learnprogramming.net",
		"codingtips.org",
		"softwareengineering.com",
	}

	title := sampleTitles[index%len(sampleTitles)]
	if index >= len(sampleTitles) {
		title = fmt.Sprintf("%s - Part %d", title, (index/len(sampleTitles))+1)
	}

	domain := sampleDomains[index%len(sampleDomains)]
	randomID := p.generateRandomIdentifier()
	path := fmt.Sprintf("/article/%s", randomID)
	url := fmt.Sprintf("https://%s%s", domain, path)

	content := fmt.Sprintf("Autogenerated seed post #%d\n\nThis is sample content for testing purposes. The post discusses %s and provides valuable insights.", index, title)

	const maxRetries = 10
	for range maxRetries {
		post := &database.Post{
			Title:     title,
			URL:       url,
			Content:   content,
			AuthorID:  &authorID,
			UpVotes:   0,
			DownVotes: 0,
			Score:     0,
		}

		if err := postRepo.Create(post); err != nil {
			randomID = p.generateRandomIdentifier()
			path = fmt.Sprintf("/article/%s", randomID)
			url = fmt.Sprintf("https://%s%s", domain, path)
			continue
		}

		return *post, nil
	}

	return database.Post{}, fmt.Errorf("failed to create post after %d attempts", maxRetries)
}

func (p *ParallelProcessor) createVotesForPost(voteRepo repositories.VoteRepository, users []database.User, post database.Post, avgVotesPerPost int) (int, error) {
	p.randMu.Lock()
	numVotes := p.randSource.Intn(avgVotesPerPost*2 + 1)
	p.randMu.Unlock()

	if numVotes == 0 && avgVotesPerPost > 0 {
		p.randMu.Lock()
		if p.randSource.Intn(5) > 0 {
			numVotes = 1
		}
		p.randMu.Unlock()
	}

	totalVotes := 0
	usedUsers := make(map[uint]bool)

	for i := 0; i < numVotes && len(usedUsers) < len(users); i++ {
		p.randMu.Lock()
		userIdx := p.randSource.Intn(len(users))
		p.randMu.Unlock()
		user := users[userIdx]

		if usedUsers[user.ID] {
			continue
		}
		usedUsers[user.ID] = true

		p.randMu.Lock()
		voteTypeInt := p.randSource.Intn(10)
		p.randMu.Unlock()
		var voteType database.VoteType
		if voteTypeInt < 7 {
			voteType = database.VoteUp
		} else {
			voteType = database.VoteDown
		}

		vote := &database.Vote{
			UserID: &user.ID,
			PostID: post.ID,
			Type:   voteType,
		}

		if err := voteRepo.CreateOrUpdate(vote); err != nil {
			return totalVotes, fmt.Errorf("create or update vote: %w", err)
		}

		totalVotes++
	}

	return totalVotes, nil
}

func (p *ParallelProcessor) updateSinglePostScore(postRepo repositories.PostRepository, voteRepo repositories.VoteRepository, post database.Post) error {
	upVotes, downVotes, err := getVoteCounts(voteRepo, post.ID)
	if err != nil {
		return fmt.Errorf("get vote counts: %w", err)
	}

	post.UpVotes = upVotes
	post.DownVotes = downVotes
	post.Score = upVotes - downVotes

	if err := postRepo.Update(&post); err != nil {
		return fmt.Errorf("update post: %w", err)
	}

	return nil
}
