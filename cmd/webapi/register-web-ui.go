package main

import (
	"net/http"
)


func registerWebUI(mux *http.ServeMux) {
	
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("WASAText backend is running"))
	})
}
