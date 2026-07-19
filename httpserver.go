package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Logger middleware for development feedback
func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Wrap ResponseWriter to capture the status code
		wrapped := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		
		next.ServeHTTP(wrapped, r)
		
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, wrapped.status, time.Since(start))
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.status = code
	lrw.ResponseWriter.WriteHeader(code)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		// Immediately return 204 No Content for preflight CORS requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

func main() {
	fmt.Println("Simple HTTP Server")

	portFlag := flag.Int("port", 8085, "optional, port number")
	baseWebFlag := flag.String("web", ".", "optional, web dir")

	flag.Parse()

	addr := fmt.Sprintf(":%d", *portFlag)
	log.Printf("Serving files in the %s directory on http://localhost%s...\n", *baseWebFlag, addr)

	// Build handler chain: CORS -> Request Logger -> Static File Server
	handler := cors(logger(http.FileServer(http.Dir(*baseWebFlag))))

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("ListenAndServe error: %v", err)
	}
}
