package asynqmon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

const (
	defaultQueuePageSize  = 10
	maxQueuePageSize      = 100
	queueInspectorWorkers = 10
	taskLookupTimeout     = 15 * time.Second
)

type queuePage struct {
	Page  int `json:"page"`
	Size  int `json:"size"`
	Total int `json:"total"`
}

type queueSort struct {
	By  string
	Dir string
}

func queuePageFromRequestWithSort(r *http.Request) (queuePage, string, queueSort, error) {
	page := queuePage{Page: 1, Size: defaultQueuePageSize}
	query := r.URL.Query()

	for key, target := range map[string]*int{"page": &page.Page, "size": &page.Size} {
		if value := query.Get(key); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 {
				return queuePage{}, "", queueSort{}, errors.New(key + " must be a positive integer")
			}
			*target = parsed
		}
	}
	if page.Size > maxQueuePageSize {
		return queuePage{}, "", queueSort{}, errors.New("size must not exceed " + strconv.Itoa(maxQueuePageSize))
	}
	sortBy := query.Get("sort_by")
	switch sortBy {
	case "", "queue", "state", "size", "memory_usage", "latency", "processed", "failed", "error_rate":
	default:
		return queuePage{}, "", queueSort{}, errors.New("unsupported sort_by")
	}
	sortDir := strings.ToLower(strings.TrimSpace(query.Get("sort_dir")))
	if sortDir == "" {
		sortDir = "asc"
	}
	if sortDir != "asc" && sortDir != "desc" {
		return queuePage{}, "", queueSort{}, errors.New("sort_dir must be asc or desc")
	}
	return page, strings.ToLower(strings.TrimSpace(query.Get("search"))), queueSort{By: sortBy, Dir: sortDir}, nil
}

func filterQueueNames(qnames []string, search string) []string {
	filtered := make([]string, 0, len(qnames))
	for _, qname := range qnames {
		if strings.Contains(strings.ToLower(qname), search) {
			filtered = append(filtered, qname)
		}
	}
	sort.Strings(filtered)
	return filtered
}

func queuePageFromRequest(r *http.Request) (queuePage, string, error) {
	page, search, _, err := queuePageFromRequestWithSort(r)
	return page, search, err
}

func selectQueuePage(qnames []string, page queuePage, search string) ([]string, queuePage) {
	filtered := filterQueueNames(qnames, search)
	page.Total = len(filtered)
	start := (page.Page - 1) * page.Size
	if start >= len(filtered) {
		return []string{}, page
	}
	end := min(start+page.Size, len(filtered))
	return filtered[start:end], page
}

func paginateQueues[T any](items []T, page queuePage) ([]T, queuePage) {
	page.Total = len(items)
	start := (page.Page - 1) * page.Size
	if start >= len(items) {
		return []T{}, page
	}
	end := min(start+page.Size, len(items))
	return items[start:end], page
}

func taskIDFromRequest(r *http.Request) (string, error) {
	taskID := strings.TrimSpace(r.URL.Query().Get("task_id"))
	if taskID == "" {
		return "", nil
	}
	if len(taskID) > 512 || strings.ContainsAny(taskID, "*?[]\\") {
		return "", errors.New("task_id contains unsupported characters")
	}
	return taskID, nil
}

func queueNamesForTaskID(ctx context.Context, rc redis.UniversalClient, taskID string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, taskLookupTimeout)
	defer cancel()

	pattern := "asynq:{*}:t:" + taskID
	suffix := "}:t:" + taskID
	matches := make(map[string]struct{})
	var cursor uint64
	for {
		keys, nextCursor, err := rc.Scan(ctx, cursor, pattern, 100_000).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			qname, ok := strings.CutPrefix(key, "asynq:{")
			if !ok {
				continue
			}
			qname, ok = strings.CutSuffix(qname, suffix)
			if ok {
				matches[qname] = struct{}{}
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	qnames := make([]string, 0, len(matches))
	for qname := range matches {
		qnames = append(qnames, qname)
	}
	return qnames, nil
}

func queuesForRequest(ctx context.Context, inspector *asynq.Inspector, rc redis.UniversalClient, page queuePage, search, taskID string) ([]string, queuePage, error) {
	qnames, err := inspector.Queues()
	if err != nil {
		return nil, queuePage{}, err
	}
	if taskID == "" {
		qnames = filterQueueNames(qnames, search)
		page.Total = len(qnames)
		return qnames, page, nil
	}

	taskQueues, err := queueNamesForTaskID(ctx, rc, taskID)
	if err != nil {
		return nil, queuePage{}, err
	}
	matchedQueues := make(map[string]struct{}, len(taskQueues))
	for _, qname := range taskQueues {
		matchedQueues[qname] = struct{}{}
	}
	filtered := qnames[:0]
	for _, qname := range qnames {
		if _, ok := matchedQueues[qname]; ok {
			filtered = append(filtered, qname)
		}
	}
	qnames = filterQueueNames(filtered, search)
	page.Total = len(qnames)
	return qnames, page, nil
}

func sortQueueSnapshots(snapshots []*queueStateSnapshot, order queueSort) {
	sort.SliceStable(snapshots, func(i, j int) bool {
		a, b := snapshots[i], snapshots[j]
		var less bool
		switch order.By {
		case "state":
			less = !a.Paused && b.Paused
		case "size":
			less = a.Size < b.Size
		case "memory_usage":
			less = a.MemoryUsage < b.MemoryUsage
		case "latency":
			less = a.LatencyMillisec < b.LatencyMillisec
		case "processed":
			less = a.Processed < b.Processed
		case "failed":
			less = a.Failed < b.Failed
		case "error_rate":
			less = errorRate(a.Failed, a.Processed) < errorRate(b.Failed, b.Processed)
		default:
			less = a.Queue < b.Queue
		}
		if a.Queue == b.Queue {
			return false
		}
		if sortQueueSnapshotsEqual(a, b, order) {
			return a.Queue < b.Queue
		}
		if order.Dir == "desc" {
			return !less
		}
		return less
	})
}

func errorRate(failed, processed int) float64 {
	if processed == 0 {
		return 0
	}
	return float64(failed) / float64(processed)
}

func sortQueueSnapshotsEqual(a, b *queueStateSnapshot, order queueSort) bool {
	switch order.By {
	case "state":
		return a.Paused == b.Paused
	case "size":
		return a.Size == b.Size
	case "memory_usage":
		return a.MemoryUsage == b.MemoryUsage
	case "latency":
		return a.LatencyMillisec == b.LatencyMillisec
	case "processed":
		return a.Processed == b.Processed
	case "failed":
		return a.Failed == b.Failed
	case "error_rate":
		return errorRate(a.Failed, a.Processed) == errorRate(b.Failed, b.Processed)
	default:
		return a.Queue == b.Queue
	}
}

func queueSnapshots(inspector *asynq.Inspector, qnames []string) ([]*queueStateSnapshot, error) {
	if len(qnames) == 0 {
		return []*queueStateSnapshot{}, nil
	}
	snapshots := make([]*queueStateSnapshot, len(qnames))
	jobs := make(chan int)
	errs := make(chan error, 1)
	var wg sync.WaitGroup
	workers := min(len(qnames), queueInspectorWorkers)

	for range workers {
		wg.Go(func() {
			for index := range jobs {
				qinfo, err := inspector.GetQueueInfo(qnames[index])
				if err != nil {
					select {
					case errs <- err:
					default:
					}
					continue
				}
				snapshots[index] = toQueueStateSnapshot(qinfo)
			}
		})
	}
	for i := range qnames {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	select {
	case err := <-errs:
		return nil, err
	default:
		return snapshots, nil
	}
}

// ****************************************************************************
// This file defines:
//   - http.Handler(s) for queue related endpoints
// ****************************************************************************

type listQueuesResponse struct {
	Queues []*queueStateSnapshot `json:"queues"`
	queuePage
}

func newListQueuesHandlerFunc(inspector *asynq.Inspector, rc redis.UniversalClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, search, order, err := queuePageFromRequestWithSort(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		taskID, err := taskIDFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		qnames, page, err := queuesForRequest(r.Context(), inspector, rc, page, search, taskID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		snapshots, err := queueSnapshots(inspector, qnames)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		sortQueueSnapshots(snapshots, order)
		snapshots, page = paginateQueues(snapshots, page)
		json.NewEncoder(w).Encode(listQueuesResponse{Queues: snapshots, queuePage: page})
	}
}

func newGetQueueHandlerFunc(inspector *asynq.Inspector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		qname := vars["qname"]

		payload := make(map[string]any)
		qinfo, err := inspector.GetQueueInfo(qname)
		if err != nil {
			// TODO: Check for queue not found error.
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		payload["current"] = toQueueStateSnapshot(qinfo)

		// TODO: make this n a variable
		data, err := inspector.History(qname, 10)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var dailyStats []*dailyStats
		for _, s := range data {
			dailyStats = append(dailyStats, toDailyStats(s))
		}
		payload["history"] = dailyStats
		json.NewEncoder(w).Encode(payload)
	}
}

func newHardDeleteQueueHandlerFunc(_ *asynq.Inspector, rc redis.UniversalClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		qname := vars["qname"]
		if err := hardDeleteQueue(r.Context(), rc, qname); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func hardDeleteQueue(ctx context.Context, rc redis.UniversalClient, qname string) error {
	if qname == "" {
		return errors.New("queue name is required")
	}
	if strings.ContainsAny(qname, "*?[]\\") {
		return errors.New("queue name contains unsupported characters")
	}

	pattern := "asynq:{" + qname + "}:*"
	if err := rc.SRem(ctx, "asynq:queues", qname).Err(); err != nil {
		return err
	}
	keys, err := rc.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return rc.Unlink(ctx, keys...).Err()
}

func newPauseQueueHandlerFunc(inspector *asynq.Inspector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		qname := vars["qname"]
		if err := inspector.PauseQueue(qname); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func newResumeQueueHandlerFunc(inspector *asynq.Inspector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		qname := vars["qname"]
		if err := inspector.UnpauseQueue(qname); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type listQueueStatsResponse struct {
	Stats map[string][]*dailyStats `json:"stats"`
	queuePage
}

func newListQueueStatsHandlerFunc(inspector *asynq.Inspector, rc redis.UniversalClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, search, _, err := queuePageFromRequestWithSort(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		taskID, err := taskIDFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		qnames, page, err := queuesForRequest(r.Context(), inspector, rc, page, search, taskID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		qnames, page = paginateQueues(qnames, page)
		resp := listQueueStatsResponse{Stats: make(map[string][]*dailyStats), queuePage: page}
		const numdays = 90 // Get stats for the last 90 days.
		for _, qname := range qnames {
			stats, err := inspector.History(qname, numdays)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			resp.Stats[qname] = toDailyStatsList(stats)
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
