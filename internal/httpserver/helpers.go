package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

var counter uint64

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func newJobID() string {
	n := atomic.AddUint64(&counter, 1)
	return "job-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + strconv.FormatUint(n, 10)
}
