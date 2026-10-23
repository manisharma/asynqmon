package asynqmon

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestAggregationRoutesDecodeGroupNames(t *testing.T) {
	endpoints := []struct {
		method string
		suffix string
		taskID string
	}{
		{http.MethodGet, "", ""},
		{http.MethodDelete, "/task-id", "task-id"},
		{http.MethodDelete, ":delete_all", ""},
		{http.MethodPost, ":batch_delete", ""},
		{http.MethodPost, "/task-id:run", "task-id"},
		{http.MethodPost, ":run_all", ""},
		{http.MethodPost, ":batch_run", ""},
		{http.MethodPost, "/task-id:archive", "task-id"},
		{http.MethodPost, ":archive_all", ""},
		{http.MethodPost, ":batch_archive", ""},
	}
	for _, root := range []string{"", "/monitor"} {
		for _, group := range []string{
			"plain-group",
			"dataform.googleapis.com/compilation_result_action-1582-list-asia-northeast1-a",
			"literal%2F/space +?#",
			"nested//group/../name",
		} {
			for _, endpoint := range endpoints {
				t.Run(root+"/"+group+"/"+endpoint.method+endpoint.suffix, func(t *testing.T) {
					router := muxRouter(Options{RootPath: root}, nil, nil)
					var got map[string]string
					err := router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
						path, err := route.GetPathTemplate()
						if err != nil {
							return err
						}
						if strings.Contains(path, "/aggregating_tasks") {
							route.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
								got = mux.Vars(r)
								w.WriteHeader(http.StatusNoContent)
							})
						}
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					target := root + "/api/queues/gcp:lans:1582:default:get/groups/" +
						url.PathEscape(group) + "/aggregating_tasks" + endpoint.suffix
					rec := httptest.NewRecorder()
					router.ServeHTTP(rec, httptest.NewRequest(endpoint.method, target, nil))
					if rec.Code != http.StatusNoContent {
						t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body)
					}
					want := map[string]string{"qname": "gcp:lans:1582:default:get", "gname": group}
					if endpoint.taskID != "" {
						want["task_id"] = endpoint.taskID
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("route variables = %#v, want %#v", got, want)
					}
				})
			}
		}
	}
}

func TestDecodePathVariables(t *testing.T) {
	for _, tc := range []struct {
		name string
		vars map[string]string
		want map[string]string
		code int
	}{
		{"existing identifiers", map[string]string{"qname": "queue%20name", "task_id": "task%252F+id"}, map[string]string{"qname": "queue name", "task_id": "task%2F+id"}, http.StatusNoContent},
		{"invalid encoding", map[string]string{"gname": "%invalid"}, nil, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler := decodePathVariables(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if got := mux.Vars(r); !reflect.DeepEqual(got, tc.want) {
					t.Errorf("route variables = %#v, want %#v", got, tc.want)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			rec := httptest.NewRecorder()
			req := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/", nil), tc.vars)
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.code {
				t.Errorf("status = %d, want %d", rec.Code, tc.code)
			}
			if called != (tc.code == http.StatusNoContent) {
				t.Errorf("handler called = %v", called)
			}
		})
	}
}
