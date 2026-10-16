package order_expiration

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
	"go-framework-demo/db"
	"go-framework-demo/model"
)

type scanFixture struct {
	mu       sync.Mutex
	cutoff   time.Time
	rows     []*model.Order
	queries  []db.ExpiredOrderQuery
	queryErr error
}

func (f *scanFixture) DatabaseNow(context.Context) (time.Time, error) { return f.cutoff, nil }
func (f *scanFixture) FindExpiredPage(ctx context.Context, input *db.ExpiredOrderQuery) ([]*model.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, *input)
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	var result []*model.Order
	for _, row := range f.rows {
		if row.ExpireTime.After(input.Cutoff) {
			continue
		}
		if !input.AfterExpireTime.IsZero() && (row.ExpireTime.Before(input.AfterExpireTime) || row.ExpireTime.Equal(input.AfterExpireTime) && row.ID <= input.AfterID) {
			continue
		}
		result = append(result, row)
		if len(result) == input.Limit {
			break
		}
	}
	return result, ctx.Err()
}

type closeFixture struct {
	started chan int64
	gate    chan struct{}
	mu      sync.Mutex
	calls   map[int64]int
	failID  int64
}

func (f *closeFixture) CloseIfExpired(ctx context.Context, id int64, waitLock bool) (bool, error) {
	f.mu.Lock()
	f.calls[id]++
	f.mu.Unlock()
	f.started <- id
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-f.gate:
	}
	if id == f.failID {
		return false, errors.New("fixture close failure")
	}
	return true, nil
}

func newScanFixture(count int) *scanFixture {
	cutoff := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	fixture := &scanFixture{cutoff: cutoff}
	for index := 0; index < count; index++ {
		// 后到期的订单ID反而更小，防止错误地只按ID推进。
		row := &model.Order{ID: int64(count - index), ExpireTime: cutoff.Add(-time.Hour).Add(time.Duration(index/3) * time.Second)}
		fixture.rows = append(fixture.rows, row)
	}
	sort.Slice(fixture.rows, func(i, j int) bool {
		source, target := fixture.rows[i], fixture.rows[j]
		if source.ExpireTime.Equal(target.ExpireTime) {
			return source.ID < target.ID
		}
		return source.ExpireTime.Before(target.ExpireTime)
	})
	return fixture
}

func TestScanRoundBackpressureAndCursor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := ants.NewPool(WorkerLimit, ants.WithNonblocking(true))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Release()
	source := newScanFixture(251)
	closer := &closeFixture{started: make(chan int64, 300), gate: make(chan struct{}), calls: make(map[int64]int), failID: 251}
	slots := make(chan struct{}, WorkerLimit)
	var tasks sync.WaitGroup
	done := make(chan error, 1)
	go func() { done <- runScanRound(ctx, source, closer, pool, slots, &tasks) }()
	for index := 0; index < WorkerLimit; index++ {
		select {
		case <-closer.started:
		case <-ctx.Done():
			t.Fatal("workers did not fill the pool")
		}
	}
	source.mu.Lock()
	queryCount := len(source.queries)
	source.mu.Unlock()
	if queryCount != 4 {
		t.Errorf("full pool performed %d queries; want 4 batches of 50", queryCount)
	}
	if len(slots) != WorkerLimit {
		t.Errorf("reserved slots=%d", len(slots))
	}
	close(closer.gate)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("scan did not finish")
	}
	tasks.Wait()
	if len(slots) != 0 {
		t.Fatal("capacity leaked")
	}
	closer.mu.Lock()
	defer closer.mu.Unlock()
	if len(closer.calls) != 251 {
		t.Fatalf("processed %d distinct orders; want 251", len(closer.calls))
	}
	for id, calls := range closer.calls {
		if calls != 1 {
			t.Errorf("order %d dispatched %d times", id, calls)
		}
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	for _, input := range source.queries {
		if input.Limit > ScanBatchSize || !input.Cutoff.Equal(source.cutoff) {
			t.Errorf("invalid page: %+v", input)
		}
	}
}

func TestScanRoundCancellationWhilePoolFull(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool, err := ants.NewPool(2, ants.WithNonblocking(true))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Release()
	source := newScanFixture(5)
	closer := &closeFixture{started: make(chan int64, 5), gate: make(chan struct{}), calls: make(map[int64]int)}
	slots := make(chan struct{}, 2)
	var tasks sync.WaitGroup
	done := make(chan error, 1)
	go func() { done <- runScanRound(ctx, source, closer, pool, slots, &tasks) }()
	for index := 0; index < 2; index++ {
		select {
		case <-closer.started:
		case <-time.After(3 * time.Second):
			t.Fatal("workers did not start")
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v; want cancellation", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("full pool prevented cancellation")
	}
	tasks.Wait()
	if len(slots) != 0 {
		t.Fatal("cancellation leaked capacity")
	}
}

func TestScanRoundQueryFailureReleasesCapacity(t *testing.T) {
	pool, err := ants.NewPool(WorkerLimit, ants.WithNonblocking(true))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Release()
	source := newScanFixture(1)
	source.queryErr = errors.New("database unavailable")
	slots := make(chan struct{}, WorkerLimit)
	var tasks sync.WaitGroup
	if err := runScanRound(context.Background(), source, nil, pool, slots, &tasks); !errors.Is(err, source.queryErr) {
		t.Fatalf("got %v", err)
	}
	tasks.Wait()
	if len(slots) != 0 {
		t.Fatal("query failure leaked capacity")
	}
}
