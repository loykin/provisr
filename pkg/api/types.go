// Package api contains stable HTTP wire contracts shared by transports and
// API clients. It deliberately contains no Gin handlers or storage adapters.
package api

import corehistory "github.com/loykin/provisr/core/history"

type ErrorResponse struct {
	Error string `json:"error"`
}

type OKResponse struct {
	OK bool `json:"ok"`
}

type HistoryResponse struct {
	Rows  []corehistory.Entry `json:"rows"`
	Total int                 `json:"total"`
}
