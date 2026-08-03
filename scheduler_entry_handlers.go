package asynqmon

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/gorilla/mux"

	"github.com/hibiken/asynq"
)

type listSchedulerEntriesResponse struct {
	Entries []*schedulerEntry `json:"entries"`
	queuePage
}

// ****************************************************************************
// This file defines:
//   - http.Handler(s) for scheduler entry related endpoints
// ****************************************************************************

func newListSchedulerEntriesHandlerFunc(inspector *asynq.Inspector, pf PayloadFormatter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, search, err := queuePageFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		entries, err := inspector.SchedulerEntries()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		filtered := make([]*asynq.SchedulerEntry, 0, len(entries))
		for _, entry := range entries {
			if strings.Contains(strings.ToLower(entry.ID), search) ||
				strings.Contains(strings.ToLower(entry.Spec), search) ||
				strings.Contains(strings.ToLower(entry.Task.Type()), search) {
				filtered = append(filtered, entry)
			}
		}
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].ID < filtered[j].ID
		})
		page.Total = len(filtered)
		start := (page.Page - 1) * page.Size
		if start >= len(filtered) {
			filtered = nil
		} else {
			end := min(start+page.Size, len(filtered))
			filtered = filtered[start:end]
		}
		if err := json.NewEncoder(w).Encode(listSchedulerEntriesResponse{
			Entries:   toSchedulerEntries(filtered, pf),
			queuePage: page,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

type listSchedulerEnqueueEventsResponse struct {
	Events []*schedulerEnqueueEvent `json:"events"`
}

func newListSchedulerEnqueueEventsHandlerFunc(inspector *asynq.Inspector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entryID := mux.Vars(r)["entry_id"]
		pageSize, pageNum := getPageOptions(r)
		events, err := inspector.ListSchedulerEnqueueEvents(
			entryID, asynq.PageSize(pageSize), asynq.Page(pageNum))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp := listSchedulerEnqueueEventsResponse{
			Events: toSchedulerEnqueueEvents(events),
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
