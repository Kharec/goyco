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

type indexedResult[T any] struct {
	value T
	index int
}

func processInParallel[T any](
	ctx context.Context,
	maxWorkers int,
	count int,
	processor func(index int) (T, error),
	errorPrefix string,
	progress *ProgressIndicator,
) ([]T, error) {
	results := make(chan indexedResult[T], count)
	errors := make(chan error, count)

	semaphore := make(chan struct{}, maxWorkers)
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

			value, err := processor(index + 1)
			if err != nil {
				errors <- fmt.Errorf("%s %d: %w", errorPrefix, index+1, err)
				return
			}

			results <- indexedResult[T]{value: value, index: index}
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	items := make([]T, count)
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
				return items, nil
			}
			items[result.index] = result.value
			completed++
			if progress != nil {
				progress.Update(completed)
			}
		case err := <-firstError:
			return nil, err
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout: %w", ctx.Err())
		}
	}

	return items, nil
}

func (p *ParallelProcessor) CreateUsersInParallel(userRepo repositories.UserRepository, count int, progress *ProgressIndicator) ([]database.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	return processInParallel(ctx, p.maxWorkers, count,
		func(index int) (database.User, error) {
			return p.createSingleUser(userRepo, index)
		},
		"create user",
		progress,
	)
}

func (p *ParallelProcessor) CreatePostsInParallel(postRepo repositories.PostRepository, authorID uint, count int, progress *ProgressIndicator) ([]database.Post, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	return processInParallel(ctx, p.maxWorkers, count,
		func(index int) (database.Post, error) {
			return p.createSinglePost(postRepo, authorID, index)
		},
		"create post",
		progress,
	)
}

func processItemsInParallel[T any, R any](
	ctx context.Context,
	maxWorkers int,
	items []T,
	processor func(index int, item T) (R, error),
	errorPrefix string,
	aggregator func(accumulator R, value R) R,
	initialValue R,
	progress *ProgressIndicator,
) (R, error) {
	count := len(items)
	results := make(chan indexedResult[R], count)
	errors := make(chan error, count)

	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		go func(index int, item T) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				errors <- ctx.Err()
				return
			}
			defer func() { <-semaphore }()

			value, err := processor(index, item)
			if err != nil {
				errors <- fmt.Errorf("%s %d: %w", errorPrefix, index+1, err)
				return
			}

			results <- indexedResult[R]{value: value, index: index}
		}(i, item)
	}

	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	accumulator := initialValue
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
				return accumulator, nil
			}
			accumulator = aggregator(accumulator, result.value)
			completed++
			if progress != nil {
				progress.Update(completed)
			}
		case err := <-firstError:
			return initialValue, err
		case <-ctx.Done():
			return initialValue, fmt.Errorf("timeout: %w", ctx.Err())
		}
	}

	return accumulator, nil
}

func (p *ParallelProcessor) CreateVotesInParallel(voteRepo repositories.VoteRepository, users []database.User, posts []database.Post, avgVotesPerPost int, progress *ProgressIndicator) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	return processItemsInParallel(ctx, p.maxWorkers, posts,
		func(index int, post database.Post) (int, error) {
			return p.createVotesForPost(voteRepo, users, post, avgVotesPerPost)
		},
		"create votes for post",
		func(acc, val int) int { return acc + val },
		0,
		progress,
	)
}

func processItemsInParallelNoResult[T any](
	ctx context.Context,
	maxWorkers int,
	items []T,
	processor func(index int, item T) error,
	errorFormatter func(index int, item T, err error) error,
	progress *ProgressIndicator,
) error {
	count := len(items)
	errors := make(chan error, count)
	completions := make(chan struct{}, count)

	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		go func(index int, item T) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				errors <- ctx.Err()
				return
			}
			defer func() { <-semaphore }()

			err := processor(index, item)
			if err != nil {
				if errorFormatter != nil {
					errors <- errorFormatter(index, item, err)
				} else {
					errors <- fmt.Errorf("process item %d: %w", index+1, err)
				}
				return
			}

			completions <- struct{}{}
		}(i, item)
	}

	go func() {
		wg.Wait()
		close(errors)
		close(completions)
	}()

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
		case _, ok := <-completions:
			if !ok {
				return nil
			}
			completed++
			if progress != nil {
				progress.Update(completed)
			}
		case err := <-firstError:
			return err
		case <-ctx.Done():
			return fmt.Errorf("timeout: %w", ctx.Err())
		}
	}

	return nil
}

func (p *ParallelProcessor) UpdatePostScoresInParallel(postRepo repositories.PostRepository, voteRepo repositories.VoteRepository, posts []database.Post, progress *ProgressIndicator) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	return processItemsInParallelNoResult(ctx, p.maxWorkers, posts,
		func(index int, post database.Post) error {
			return p.updateSinglePostScore(postRepo, voteRepo, post)
		},
		func(index int, post database.Post, err error) error {
			return fmt.Errorf("update post %d scores: %w", post.ID, err)
		},
		progress,
	)
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
		user := &database.User{
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
