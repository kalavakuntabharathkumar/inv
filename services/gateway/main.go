package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	catalog := env("CATALOG_URL", "http://localhost:8081")
	inventory := env("INVENTORY_URL", "http://localhost:8082")
	orders := env("ORDERS_URL", "http://localhost:8083")
	client := &http.Client{Timeout: 5 * time.Second}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"status":"ok","service":"gateway"}`)) })
	route := func(prefix, target string) {
		mux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, strings.Split(prefix, " ")[1])
			url := target + path
			if r.URL.RawQuery != "" {
				url += "?" + r.URL.RawQuery
			}
			req, err := http.NewRequestWithContext(r.Context(), r.Method, url, r.Body)
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			req.Header = r.Header.Clone()
			resp, err := client.Do(req)
			if err != nil {
				http.Error(w, "upstream unavailable", 502)
				return
			}
			defer resp.Body.Close()
			for k, values := range resp.Header {
				for _, v := range values {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(resp.StatusCode)
			_, _ = io.Copy(w, resp.Body)
		})
	}
	route("GET /api/products", catalog+"/products")
	route("POST /api/products", catalog+"/products")
	route("GET /api/inventory/", inventory+"/inventory/")
	route("POST /api/inventory/", inventory+"/inventory/")
	route("GET /api/orders", orders+"/orders")
	route("POST /api/orders", orders+"/orders")
	port := env("PORT", "8080")
	log.Printf("gateway listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
