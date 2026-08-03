package asynqmon

import (
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestQueuePageFromRequest(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   queuePage
		search string
		err    bool
	}{
		{name: "defaults", target: "/api/queues", want: queuePage{Page: 1, Size: 50}},
		{name: "parameters", target: "/api/queues?page=2&size=25&search=Payments", want: queuePage{Page: 2, Size: 25}, search: "payments"},
		{name: "zero page", target: "/api/queues?page=0", err: true},
		{name: "oversized page", target: "/api/queues?size=101", err: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, search, err := queuePageFromRequest(httptest.NewRequest("GET", tc.target, nil))
			if (err != nil) != tc.err {
				t.Fatalf("queuePageFromRequest error = %v, want error: %v", err, tc.err)
			}
			if !tc.err && (page != tc.want || search != tc.search) {
				t.Fatalf("queuePageFromRequest = (%+v, %q), want (%+v, %q)", page, search, tc.want, tc.search)
			}
		})
	}
}

func TestSelectQueuePage(t *testing.T) {
	qnames := []string{"Zulu", "alpha:orders", "alpha:payments", "beta"}
	got, page := selectQueuePage(qnames, queuePage{Page: 1, Size: 1}, "alpha")
	if want := []string{"alpha:orders"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selectQueuePage queues = %v, want %v", got, want)
	}

	if page.Total != 2 {
		t.Fatalf("selectQueuePage total = %d, want 2", page.Total)
	}

	got, page = selectQueuePage(qnames, queuePage{Page: 3, Size: 1}, "alpha")
	if len(got) != 0 || page.Total != 2 {
		t.Fatalf("selectQueuePage out-of-range = (%v, %+v), want ([], total 2)", got, page)
	}
}

func TestHardDeleteQueueRejectsUnsafeNames(t *testing.T) {
	for _, qname := range []string{"", "queue*", "queue?", "queue[1]", `queue\name`} {
		if err := hardDeleteQueue(nil, nil, qname); err == nil {
			t.Errorf("hardDeleteQueue(%q) returned nil error", qname)
		}
	}
}
