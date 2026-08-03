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

	"github.com/gorilla/mux"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

const (
	defaultQueuePageSize  = 50
	maxQueuePageSize      = 100
	queueInspectorWorkers = 10
)

type queuePage struct {
	Page  int `json:"page"`
	Size  int `json:"size"`
	Total int `json:"total"`
}

func queuePageFromRequest(r *http.Request) (queuePage, string, error) {
	page := queuePage{Page: 1, Size: defaultQueuePageSize}
	query := r.URL.Query()

	for key, target := range map[string]*int{"page": &page.Page, "size": &page.Size} {
		if value := query.Get(key); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 {
				return queuePage{}, "", errors.New(key + " must be a positive integer")
			}
			*target = parsed
		}
	}
	if page.Size > maxQueuePageSize {
		return queuePage{}, "", errors.New("size must not exceed " + strconv.Itoa(maxQueuePageSize))
	}
	return page, strings.ToLower(strings.TrimSpace(query.Get("search"))), nil
}

func selectQueuePage(qnames []string, page queuePage, search string) ([]string, queuePage) {
	filtered := make([]string, 0, len(qnames))
	for _, qname := range qnames {
		if strings.Contains(strings.ToLower(qname), search) {
			filtered = append(filtered, qname)
		}
	}
	sort.Strings(filtered)
	page.Total = len(filtered)
	start := (page.Page - 1) * page.Size
	if start >= len(filtered) {
		return []string{}, page
	}
	end := min(start+page.Size, len(filtered))
	return filtered[start:end], page
}

func queueSnapshots(inspector *asynq.Inspector, qnames []string) ([]*queueStateSnapshot, error) {
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

func newListQueuesHandlerFunc(inspector *asynq.Inspector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, search, err := queuePageFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		qnames, err := inspector.Queues()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		qnames, page = selectQueuePage(qnames, page, search)
		snapshots, err := queueSnapshots(inspector, qnames)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
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

func newListQueueStatsHandlerFunc(inspector *asynq.Inspector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, search, err := queuePageFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		qnames, err := inspector.Queues()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		qnames, page = selectQueuePage(qnames, page, search)
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
