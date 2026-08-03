package asynqmon

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/hibiken/asynq"
)

// ****************************************************************************
// This file defines:
//   - http.Handler(s) for server related endpoints
// ****************************************************************************

type listServersResponse struct {
	Servers []*serverInfo `json:"servers"`
	queuePage
}

func newListServersHandlerFunc(inspector *asynq.Inspector, pf PayloadFormatter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, search, err := queuePageFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		srvs, err := inspector.Servers()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		filtered := make([]*asynq.ServerInfo, 0, len(srvs))
		for _, srv := range srvs {
			if strings.Contains(strings.ToLower(srv.Host), search) {
				filtered = append(filtered, srv)
			}
		}
		sort.Slice(filtered, func(i, j int) bool {
			if filtered[i].Host == filtered[j].Host {
				return filtered[i].PID < filtered[j].PID
			}
			return filtered[i].Host < filtered[j].Host
		})
		page.Total = len(filtered)
		start := (page.Page - 1) * page.Size
		if start >= len(filtered) {
			filtered = nil
		} else {
			end := min(start+page.Size, len(filtered))
			filtered = filtered[start:end]
		}
		resp := listServersResponse{
			Servers:   toServerInfoList(filtered, pf),
			queuePage: page,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
